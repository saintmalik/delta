package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/saintmalik/delta/handlers"
)

func main() {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Get("/", handlers.HandleMain)
		r.Post("/signup", handlers.HandleSignup)
		r.Get("/callback", handlers.HandleGitHubCallback)
    })

	r.Group(func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(handlers.IsAuthenticated)
		r.Get("/package", handlers.AddPackage)
		r.Post("/package", handlers.AddPackage)
		r.Get("/dash", handlers.ListPackage)
		r.Post("/logout", handlers.HandleUserLogout)
		r.Get("/check", handlers.CheckReleasesHandler)
    })

	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, "public"))
	FileServer(r, "/assets", filesDir)

	http.ListenAndServe(":4000", r)
}

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
