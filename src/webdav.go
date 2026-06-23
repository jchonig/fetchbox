package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type webDAVUploader struct {
	baseURL  string
	username string
	password string
}

// newWebDAVUploader constructs an uploader from a Storage definition and a per-folder path.
// s.URL must use the webdavs:// or webdav:// scheme with an optional embedded username,
// e.g. webdavs://user@host/remote.php/webdav/base/
func newWebDAVUploader(s *Storage, path string) (FileUploader, error) {
	if s.Type != "webdav" {
		return nil, fmt.Errorf("unsupported storage type %q", s.Type)
	}

	parsed, err := url.Parse(s.URL)
	if err != nil {
		return nil, fmt.Errorf("parse storage url: %w", err)
	}

	var scheme string
	switch parsed.Scheme {
	case "webdavs":
		scheme = "https"
	case "webdav":
		scheme = "http"
	default:
		return nil, fmt.Errorf("storage url must use webdav:// or webdavs:// scheme, got %q", parsed.Scheme)
	}

	username := ""
	if parsed.User != nil {
		username = parsed.User.Username()
	}

	basePath := strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	httpURL := &url.URL{Scheme: scheme, Host: parsed.Host, Path: basePath}

	return &webDAVUploader{
		baseURL:  strings.TrimRight(httpURL.String(), "/"),
		username: username,
		password: s.Password(),
	}, nil
}

func (w *webDAVUploader) Upload(filename string, data []byte) error {
	putURL := w.baseURL + "/" + filename

	req, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	if w.username != "" {
		req.SetBasicAuth(w.username, w.password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("put %s: %w", putURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("put %s: HTTP %d", putURL, resp.StatusCode)
	}

	return nil
}
