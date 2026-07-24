package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/m1iktea/hearth/server/internal/discovery"
	"github.com/m1iktea/hearth/server/internal/store"
)

func NewRouter(snaps *store.SnapshotStore, nav *store.NavStore, inventory *store.InventoryStore, scanner *discovery.ARPScanner, dist fs.FS, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, "ok")
	})

	// 能力开关：前端据此隐藏不可用的功能入口（如 ARP 扫描按钮）
	mux.HandleFunc("GET /api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, map[string]bool{"arp_discovery": scanner != nil})
	})

	sh := &statusHandler{snaps: snaps}
	mux.HandleFunc("GET /api/v1/status", sh.all)
	mux.HandleFunc("GET /api/v1/status/{source}", sh.bySource)

	// typed-nil 归一：*ARPScanner 的 nil 直接塞进接口后接口不等于 nil，
	// 会绕过 handler 的未启用保护并在 nil 接收者上 panic
	var arpScan arpScanner
	if scanner != nil {
		arpScan = scanner
	}

	registerNavRoutes(mux, nav, inventory)
	registerInventoryRoutes(mux, inventory, nav, arpScan)
	registerMetricsRoutes(mux, inventory)
	registerHealthTimelineRoutes(mux, inventory)

	mux.Handle("/", spaHandler(dist))

	return withMiddleware(mux, logger)
}

// withMiddleware: 日志 + auth 插槽（MVP 为直通，后续在此接入认证）。
func withMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return requestLogger(authStub(next), logger)
}

func authStub(next http.Handler) http.Handler { return next }

func requestLogger(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug("http", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}
