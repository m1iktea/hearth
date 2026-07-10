package api

import (
	"net/http"

	"github.com/m1iktea/hearth/server/internal/store"
)

func registerNavRoutes(mux *http.ServeMux, nav *store.NavStore) {
	_ = nav // Task 10 实现
	_ = mux
}
