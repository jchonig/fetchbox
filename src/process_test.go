package main

import (
	"errors"
	"testing"
)

// fakeFetcher is a test double for MailFetcher.
type fakeFetcher struct {
	msgs     map[string][]RawMessage
	seen     map[string][]uint32
	deleted  map[string][]uint32
	fetchErr error
	markErr  error
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{
		msgs:    make(map[string][]RawMessage),
		seen:    make(map[string][]uint32),
		deleted: make(map[string][]uint32),
	}
}

func (f *fakeFetcher) Fetch(folder string, _ bool) ([]RawMessage, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.msgs[folder], nil
}

func (f *fakeFetcher) DeleteMessages(folder string, uids []uint32) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.deleted[folder] = append(f.deleted[folder], uids...)
	return nil
}

func (f *fakeFetcher) MarkSeen(folder string, uids []uint32) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.seen[folder] = append(f.seen[folder], uids...)
	return nil
}

func (f *fakeFetcher) Close() error { return nil }

// fakeUploader is a test double for FileUploader.
type fakeUploader struct {
	uploads []uploadedFile
	err     error
}

type uploadedFile struct {
	name string
	data []byte
}

func (u *fakeUploader) Upload(name string, data []byte) error {
	if u.err != nil {
		return u.err
	}
	u.uploads = append(u.uploads, uploadedFile{name: name, data: data})
	return nil
}

var noopLogger = &logger{}

func TestProcessFolderEmpty(t *testing.T) {
	fetcher := newFakeFetcher()
	uploader := &fakeUploader{}

	if err := processFolder(fetcher, "INBOX", false, uploader, false, noopLogger); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uploader.uploads) != 0 {
		t.Errorf("expected no uploads, got %d", len(uploader.uploads))
	}
	if len(fetcher.seen["INBOX"]) != 0 {
		t.Errorf("expected no seen marks, got %d", len(fetcher.seen["INBOX"]))
	}
}

func TestProcessFolderNoop(t *testing.T) {
	fetcher := newFakeFetcher()
	fetcher.msgs["INBOX"] = []RawMessage{
		{UID: 1, Data: buildMessage("test.txt", []byte("hello"))},
	}
	uploader := &fakeUploader{}

	if err := processFolder(fetcher, "INBOX", false, uploader, true, noopLogger); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uploader.uploads) != 0 {
		t.Errorf("noop: expected no uploads, got %d", len(uploader.uploads))
	}
	if len(fetcher.seen["INBOX"]) != 0 {
		t.Errorf("noop: expected no seen marks, got %d", len(fetcher.seen["INBOX"]))
	}
}

func TestProcessFolderWithAttachment(t *testing.T) {
	fetcher := newFakeFetcher()
	want := []byte("attachment contents")
	fetcher.msgs["INBOX"] = []RawMessage{
		{UID: 42, Data: buildMessage("report.pdf", want)},
	}
	uploader := &fakeUploader{}

	if err := processFolder(fetcher, "INBOX", false, uploader, false, noopLogger); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(uploader.uploads) != 1 {
		t.Fatalf("uploads: got %d, want 1", len(uploader.uploads))
	}
	if uploader.uploads[0].name != "report.pdf" {
		t.Errorf("filename: got %q, want %q", uploader.uploads[0].name, "report.pdf")
	}
	if string(uploader.uploads[0].data) != string(want) {
		t.Errorf("data mismatch")
	}
	if len(fetcher.seen["INBOX"]) != 1 || fetcher.seen["INBOX"][0] != 42 {
		t.Errorf("seen UIDs: got %v, want [42]", fetcher.seen["INBOX"])
	}
}

func TestProcessFolderDeleteAfter(t *testing.T) {
	fetcher := newFakeFetcher()
	want := []byte("attachment contents")
	fetcher.msgs["INBOX"] = []RawMessage{
		{UID: 7, Data: buildMessage("doc.pdf", want)},
	}
	uploader := &fakeUploader{}

	if err := processFolder(fetcher, "INBOX", true, uploader, false, noopLogger); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uploader.uploads) != 1 {
		t.Fatalf("uploads: got %d, want 1", len(uploader.uploads))
	}
	if len(fetcher.seen["INBOX"]) != 0 {
		t.Errorf("should not mark seen when delete_after, got %v", fetcher.seen["INBOX"])
	}
	if len(fetcher.deleted["INBOX"]) != 1 || fetcher.deleted["INBOX"][0] != 7 {
		t.Errorf("deleted UIDs: got %v, want [7]", fetcher.deleted["INBOX"])
	}
}

func TestProcessFolderFetchError(t *testing.T) {
	fetcher := newFakeFetcher()
	fetcher.fetchErr = errors.New("imap failure")
	uploader := &fakeUploader{}

	err := processFolder(fetcher, "INBOX", false, uploader, false, noopLogger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProcessFolderUploadError(t *testing.T) {
	fetcher := newFakeFetcher()
	fetcher.msgs["INBOX"] = []RawMessage{
		{UID: 1, Data: buildMessage("doc.txt", []byte("data"))},
	}
	uploader := &fakeUploader{err: errors.New("webdav 500")}

	// Upload error should not propagate as a hard error, but uid should not be marked seen
	if err := processFolder(fetcher, "INBOX", false, uploader, false, noopLogger); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fetcher.seen["INBOX"]) != 0 {
		t.Errorf("should not mark seen on upload failure, got %v", fetcher.seen["INBOX"])
	}
}

func TestExtractAttachmentsEncodedFilename(t *testing.T) {
	// "Report.pdf" encoded as =?UTF-8?B?...?= (RFC 2047 base64)
	data := []byte("file contents")
	raw := buildMessageRawFilename(`"=?UTF-8?B?UmVwb3J0LnBkZg==?="`, data)

	atts, err := extractAttachments(raw)
	if err != nil {
		t.Fatalf("extractAttachments: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(atts))
	}
	if atts[0].Filename != "Report.pdf" {
		t.Errorf("filename: got %q, want %q", atts[0].Filename, "Report.pdf")
	}
}

func TestExtractAttachmentsNone(t *testing.T) {
	msg := "From: a@b.com\r\nTo: c@d.com\r\nSubject: hi\r\nContent-Type: text/plain\r\n\r\nbody\r\n"
	atts, err := extractAttachments([]byte(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(atts))
	}
}

func TestExtractAttachmentsOne(t *testing.T) {
	data := []byte("file contents")
	raw := buildMessage("hello.txt", data)

	atts, err := extractAttachments(raw)
	if err != nil {
		t.Fatalf("extractAttachments: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(atts))
	}
	if atts[0].Filename != "hello.txt" {
		t.Errorf("filename: got %q, want %q", atts[0].Filename, "hello.txt")
	}
	if string(atts[0].Data) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", atts[0].Data, data)
	}
}
