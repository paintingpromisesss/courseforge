//go:build pprof

package api

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
)

func registerPprof(r chi.Router) {
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Get("/debug/pprof/{name}", func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler(chi.URLParam(r, "name")).ServeHTTP(w, r)
	})
}
