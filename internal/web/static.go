package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:bili-up-web
var staticFiles embed.FS

// GetStaticFS 返回嵌入的静态文件系统
func GetStaticFS() fs.FS {
	// 返回 bili-up-web/out 子目录（Next.js 静态导出目录）
	sub, err := fs.Sub(staticFiles, "bili-up-web/out")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}
	return sub
}

// StaticFileHandler 创建一个处理静态文件的 HTTP 处理器
func StaticFileHandler() http.Handler {
	staticFS := GetStaticFS()
	// 使用标准库的 FileServer，它会自动处理 Content-Type、Range 等
	return &staticHandler{
		fs:         staticFS,
		fileServer: http.FileServer(http.FS(staticFS)),
	}
}

type staticHandler struct {
	fs         fs.FS
	fileServer http.Handler
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// 如果路径为空，默认访问 index.html
	if path == "" {
		path = "index.html"
	}

	// 尝试打开文件
	f, err := h.fs.Open(path)

	// 如果打开失败（文件不存在）
	if err != nil {
		// 对于 API 或 _next 路径，直接返回 404
		if strings.HasPrefix(path, "api/") || strings.HasPrefix(path, "_next/") {
			http.NotFound(w, r)
			return
		}

		// SPA 路由处理：按优先级尝试多种路径
		// 去掉末尾斜杠进行尝试
		cleanPath := strings.TrimSuffix(path, "/")
		found := false

		// 1. 尝试 path.html (例如 settings -> settings.html)
		if cleanPath != "" && !strings.HasSuffix(cleanPath, ".html") {
			htmlPath := cleanPath + ".html"
			if f2, err2 := h.fs.Open(htmlPath); err2 == nil {
				f = f2
				path = htmlPath
				found = true
			}
		}

		// 2. 尝试 path/index.html (例如 settings/ -> settings/index.html)
		if !found && cleanPath != "" {
			indexPath := cleanPath + "/index.html"
			if f2, err2 := h.fs.Open(indexPath); err2 == nil {
				f = f2
				path = indexPath
				found = true
			}
		}

		// 3. 都不存在，回退到 index.html (SPA Entry)
		if !found {
			path = "index.html"
			f, err = h.fs.Open(path)
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
		}
	}
	defer f.Close()

	d, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 如果是目录，尝试 index.html
	if d.IsDir() {
		// close current file
		f.Close()
		path += "/index.html"
		f, err = h.fs.Open(path)
		if err != nil {
			// 目录没有 index.html，且不是 API/_next，回退到全局 index.html
			if !strings.HasPrefix(path, "api/") && !strings.HasPrefix(path, "_next/") {
				path = "index.html"
				f, err = h.fs.Open(path)
				if err != nil {
					http.NotFound(w, r)
					return
				}
			} else {
				http.NotFound(w, r)
				return
			}
		}
		// update stat
		d, err = f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// 使用 ServeContent 自动处理 MIME 和 Caching
	http.ServeContent(w, r, path, d.ModTime(), f.(io.ReadSeeker))
}
