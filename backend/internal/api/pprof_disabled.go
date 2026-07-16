//go:build !pprof

package api

import "github.com/go-chi/chi/v5"

func registerPprof(_ chi.Router) {}
