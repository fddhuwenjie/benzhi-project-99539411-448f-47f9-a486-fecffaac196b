package web

import (
	"embed"
	"net/http"
)

//go:embed static/index.html static/app.css static/app.js
var assets embed.FS

func (s *Server) HandleWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "页面不存在", "")
		return
	}
	serveAsset(w, "static/index.html", "text/html; charset=utf-8")
}

func (s *Server) HandleCSS(w http.ResponseWriter, _ *http.Request) {
	serveAsset(w, "static/app.css", "text/css; charset=utf-8")
}
func (s *Server) HandleJS(w http.ResponseWriter, _ *http.Request) {
	serveAsset(w, "static/app.js", "text/javascript; charset=utf-8")
}

func serveAsset(w http.ResponseWriter, path, contentType string) {
	b, err := assets.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ASSET_ERROR", "页面资源读取失败", "")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}
