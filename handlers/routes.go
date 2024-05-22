package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/saintmalik/delta/views"
)

func HandleMain(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method == "GET" {
		templ.Handler(views.HomePage()).ServeHTTP(w, r)
	}

}