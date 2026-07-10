package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// spaHandler 托管前端构建产物；未命中的非 /api 路径回落到 index.html（SPA 路由）。
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(dist, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/" // fallback to index.html
		fileServer.ServeHTTP(w, r)
	})
}
