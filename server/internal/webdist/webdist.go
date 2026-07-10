package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Dist 返回前端构建产物；本地开发时为占位页，Docker 构建时为真实前端。
func Dist() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
