//go:build !swagger

package api

import "github.com/go-chi/chi/v5"

func registerSwagger(_ chi.Router) {}
