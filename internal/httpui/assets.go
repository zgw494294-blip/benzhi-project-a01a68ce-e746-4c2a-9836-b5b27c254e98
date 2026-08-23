package httpui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/index.html assets/app.css assets/app.js
var assetFS embed.FS

func (s *Server) registerAssets() {
	assets, _ := fs.Sub(assetFS, "assets")
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
}
