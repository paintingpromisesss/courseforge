package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns an fs.FS sub-filesystem rooted at dist.
func Dist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// HasEmbedded reports whether a valid frontend build (index.html) is embedded.
func HasEmbedded() bool {
	f, err := Dist().Open("index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
