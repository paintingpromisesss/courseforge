package handlers

import (
	"net/http"
	"os"
	"strings"
)

// queryBool reads a truthy query param ("1" or "true", case-insensitive).
func queryBool(r *http.Request, key string) bool {
	v := r.URL.Query().Get(key)
	return v == "1" || strings.EqualFold(v, "true")
}

func serveTextFile(w http.ResponseWriter, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to read file", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}
