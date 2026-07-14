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
	"github.com/m1iktea/hearth/server/internal/discovery"
	"github.com/m1iktea/hearth/server/internal/health"
	"github.com/m1iktea/hearth/server/internal/metrics"
	"github.com/m1iktea/hearth/server/internal/sensors"
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
	inventory, err := store.OpenInventory(filepath.Join(cfg.DataDir, "hearth.db"))
	if err != nil {
		return err
	}
	defer inventory.Close()

	dist, err := webdist.Dist()
	if err != nil {
		return err
	}

	// 黑匣子：恢复上次持久化的快照，让重启后的第一屏就有“最后已知状态”。
	if restored, err := inventory.LoadSnapshots(); err != nil {
		logger.Warn("restore snapshots failed", "error", err)
	} else {
		for _, snap := range restored {
			snaps.Restore(snap)
		}
		if len(restored) > 0 {
			logger.Info("snapshots restored", "count", len(restored))
		}
	}
	recorder := metrics.NewRecorder(snaps, inventory, cfg.MetricSampleInterval, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sched := collector.NewScheduler(collectors, recorder, cfg.PollInterval, logger)
	go sched.Run(ctx)
	go health.NewRunner(inventory, cfg.HealthInterval, logger).Run(ctx)
	go runRetention(ctx, inventory, cfg, logger)
	if cfg.PVESensorsEnabled() {
		prober, err := sensors.NewProber(sensors.Config{
			Addr: cfg.PVESSHHost, User: cfg.PVESSHUser,
			Password: cfg.PVESSHPassword, KeyFile: cfg.PVESSHKeyFile,
		}, inventory, cfg.MetricSampleInterval, logger)
		if err != nil {
			return err
		}
		go prober.Run(ctx)
		logger.Info("pve temperature probe enabled", "host", cfg.PVESSHHost)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewRouter(snaps, nav, inventory, arpScanner(cfg), dist, logger),
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

// runRetention 启动时立即清理一次过期事件/指标，之后每 12 小时清理一次。
func runRetention(ctx context.Context, inventory *store.InventoryStore, cfg *config.Config, logger *slog.Logger) {
	prune := func() {
		now := time.Now().UTC()
		n, err := inventory.PruneBefore(now.Add(-cfg.EventRetention), now.Add(-cfg.MetricRetention))
		if err != nil {
			logger.Warn("retention prune failed", "error", err)
			return
		}
		if n > 0 {
			logger.Info("retention pruned", "rows", n)
		}
	}
	prune()
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}

func arpScanner(cfg *config.Config) *discovery.ARPScanner {
	if !cfg.ARPDiscoveryEnabled {
		return nil
	}
	return discovery.NewARPScanner(cfg.ScanNetworks)
}
