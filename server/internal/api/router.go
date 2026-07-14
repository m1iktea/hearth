package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/m1iktea/hearth/server/internal/store"
)

func NewRouter(snaps *store.SnapshotStore, nav *store.NavStore, inventory *store.InventoryStore, arpScanner arpScanner, dist fs.FS, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, "ok")
	})

	sh := &statusHandler{snaps: snaps}
	mux.HandleFunc("GET /api/v1/status", sh.all)
	mux.HandleFunc("GET /api/v1/status/{source}", sh.bySource)

	registerNavRoutes(mux, nav) // Task 10 实现；本 Task 先提供空实现避免编译失败
	registerInventoryRoutes(mux, inventory, arpScanner)

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
