package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/m1iktea/hearth/server/internal/store"
)

// registerHealthTimelineRoutes 暴露某检查的状态迁移事件，供前端还原绿红可用率时间线。
// 沿用 metrics 端点的参数风格：since（RFC3339，默认最近 24h）+ limit。
func registerHealthTimelineRoutes(mux *http.ServeMux, inventory *store.InventoryStore) {
	mux.HandleFunc("GET /api/v1/health/timeline", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checkID, err := strconv.ParseInt(q.Get("check_id"), 10, 64)
		if err != nil || checkID <= 0 {
			writeError(w, http.StatusBadRequest, "check_id 必须为正整数")
			return
		}
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
		items, err := inventory.QueryHealthTransitions(checkID, since, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "查询健康时间线失败")
			return
		}
		writeOK(w, items)
	})
}
