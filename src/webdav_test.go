package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebDAVUpload(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   []byte
		gotAuth   string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	// Build a webdav:// URL pointing at the test server (http underneath).
	// httptest.Server uses http, so we use webdav:// → http.
	stor := &Storage{
		Type:        "webdav",
		URL:         "webdav://user@" + srv.Listener.Addr().String() + "/",
		PasswordEnv: "",
	}

	u, err := newWebDAVUploader(stor, "/files/")
	if err != nil {
		t.Fatalf("newWebDAVUploader: %v", err)
	}

	payload := []byte("hello webdav")
	if err := u.Upload("doc.txt", payload); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method: got %q, want PUT", gotMethod)
	}
	if gotPath != "/files/doc.txt" {
		t.Errorf("path: got %q, want /files/doc.txt", gotPath)
	}
	if string(gotBody) != string(payload) {
		t.Errorf("body: got %q, want %q", gotBody, payload)
	}
	if gotAuth == "" {
		t.Error("expected Authorization header")
	}
}

func TestWebDAVUploadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	stor := &Storage{
		Type: "webdav",
		URL:  "webdav://" + srv.Listener.Addr().String() + "/",
	}
	u, err := newWebDAVUploader(stor, "/")
	if err != nil {
		t.Fatalf("newWebDAVUploader: %v", err)
	}

	err = u.Upload("file.txt", []byte("data"))
	if err == nil {
		t.Fatal("expected error for HTTP 403, got nil")
	}
}

func TestWebDAVUnsupportedType(t *testing.T) {
	stor := &Storage{Type: "s3", URL: "s3://bucket/"}
	_, err := newWebDAVUploader(stor, "/")
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestWebDAVBadScheme(t *testing.T) {
	stor := &Storage{Type: "webdav", URL: "https://host/path"}
	_, err := newWebDAVUploader(stor, "/")
	if err == nil {
		t.Fatal("expected error for non-webdav scheme, got nil")
	}
}
