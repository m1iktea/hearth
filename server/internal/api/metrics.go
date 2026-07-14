package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/m1iktea/hearth/server/internal/store"
)

// registerMetricsRoutes 暴露黑匣子指标查询，用于故障后回看崩溃前的资源走势。
func registerMetricsRoutes(mux *http.ServeMux, inventory *store.InventoryStore) {
	mux.HandleFunc("GET /api/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		since := time.Now().UTC().Add(-24 * time.Hour)
		if raw := q.Get("since"); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "since 必须为 RFC3339 时间格式")
				return
			}
			since = t
		}
		limit := 0
		if raw := q.Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "limit 必须为整数")
				return
			}
			limit = n
		}
		samples, err := inventory.QuerySamples(q.Get("source"), q.Get("object"), q.Get("metric"), since, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "查询指标失败")
			return
		}
		writeOK(w, samples)
	})
}
