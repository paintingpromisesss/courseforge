package handlers

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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

func archiveFormat(url string) string {
	u := strings.ToLower(url)
	switch {
	case strings.HasSuffix(u, ".tar.gz") || strings.HasSuffix(u, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(u, ".tar.bz2") || strings.HasSuffix(u, ".tbz2"):
		return "tar.bz2"
	case strings.HasSuffix(u, ".zip"):
		return "zip"
	default:
		return ""
	}
}

func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	return extractTar(gz, destDir)
}

func extractTarBz2(r io.Reader, destDir string) error {
	return extractTar(bzip2.NewReader(r), destDir)
}

func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		clean := filepath.Join(destDir, filepath.Clean("/" + hdr.Name)[1:])
		if !strings.HasPrefix(clean, destDir+string(os.PathSeparator)) && clean != destDir {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(clean, 0755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(clean), 0755)
			f, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			f.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = os.Symlink(hdr.Linkname, clean)
		}
	}
	return nil
}

func extractZip(f *os.File, destDir string) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	for _, zf := range zr.File {
		clean := filepath.Join(destDir, filepath.Clean("/" + zf.Name)[1:])
		if !strings.HasPrefix(clean, destDir+string(os.PathSeparator)) && clean != destDir {
			continue
		}
		if zf.FileInfo().IsDir() {
			_ = os.MkdirAll(clean, 0755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(clean), 0755)
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, zf.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
