package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/m1iktea/hearth/server/internal/api"
	"github.com/m1iktea/hearth/server/internal/collector"
	dockercol "github.com/m1iktea/hearth/server/internal/collector/docker"
	"github.com/m1iktea/hearth/server/internal/collector/openwrt"
	"github.com/m1iktea/hearth/server/internal/collector/proxmox"
	"github.com/m1iktea/hearth/server/internal/config"
	"github.com/m1iktea/hearth/server/internal/store"
	"github.com/m1iktea/hearth/server/internal/webdist"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	var collectors []collector.Collector
	if cfg.PVEEnabled() {
		collectors = append(collectors, proxmox.New(cfg.PVEURL, cfg.PVETokenID, cfg.PVETokenSecret))
	}
	dc, err := dockercol.New(cfg.DockerHost)
	if err != nil {
		return err
	}
	collectors = append(collectors, dc)
	if cfg.OpenWrtEnabled() {
		collectors = append(collectors, openwrt.New(cfg.OpenWrtURL, cfg.OpenWrtUsername, cfg.OpenWrtPassword))
	}

	snaps := store.NewSnapshotStore()
	nav, err := store.OpenNav(filepath.Join(cfg.DataDir, "hearth.db"))
	if err != nil {
		return err
	}
	defer nav.Close()

	dist, err := webdist.Dist()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sched := collector.NewScheduler(collectors, snaps, cfg.PollInterval, logger)
	go sched.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewRouter(snaps, nav, dist, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("hearth started", "addr", cfg.ListenAddr, "sources", len(collectors), "interval", cfg.PollInterval)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
