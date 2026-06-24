package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"

	"github.com/emersion/go-message/mail"
)

// MailFetcher abstracts IMAP operations for testability.
type MailFetcher interface {
	Fetch(folder string, unseenOnly bool) ([]RawMessage, error)
	MarkSeen(folder string, uids []uint32) error
	DeleteMessages(folder string, uids []uint32) error
	Close() error
}

// FileUploader abstracts file persistence for testability.
type FileUploader interface {
	Upload(filename string, data []byte) error
}

// RawMessage holds a UID and raw RFC 5322 message bytes.
type RawMessage struct {
	UID  uint32
	Data []byte
}

// Attachment is a decoded MIME attachment.
type Attachment struct {
	Filename string
	Data     []byte
}

type processor struct {
	cfg    *Config
	noop   bool
	logger *logger
}

func (p *processor) run() {
	for _, mb := range p.cfg.Mailboxes {
		if err := p.processMailbox(mb); err != nil {
			log.Printf("mailbox %s: %v", mb.Name, err)
		}
	}
}

func (p *processor) processMailbox(mb Mailbox) error {
	p.logger.infof("[%s] connecting to %s:%d", mb.Name, mb.Host, mb.Port)
	client, err := newIMAPClient(mb, p.logger)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()
	p.logger.infof("[%s] logged in as %s", mb.Name, mb.Username)

	for _, folder := range mb.Folders {
		stor, ok := p.cfg.Storage[folder.Storage]
		if !ok {
			log.Printf("  folder %s: unknown storage %q", folder.Name, folder.Storage)
			continue
		}
		uploader, err := newWebDAVUploader(stor, folder.Path)
		if err != nil {
			log.Printf("  folder %s: storage error: %v", folder.Name, err)
			continue
		}
		if err := processFolder(client, folder.Name, folder.DeleteAfter, uploader, p.noop, p.logger); err != nil {
			log.Printf("  folder %s: %v", folder.Name, err)
		}
	}
	return nil
}

func processFolder(fetcher MailFetcher, folder string, deleteAfter bool, uploader FileUploader, noop bool, l *logger) error {
	msgs, err := fetcher.Fetch(folder, !deleteAfter)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	if deleteAfter {
		l.infof("[%s] found %d message(s)", folder, len(msgs))
	} else {
		l.infof("[%s] found %d unseen message(s)", folder, len(msgs))
	}

	if len(msgs) == 0 {
		return nil
	}

	var processed []uint32

	for _, msg := range msgs {
		attachments, err := extractAttachments(msg.Data)
		if err != nil {
			log.Printf("  uid %d: parse error: %v", msg.UID, err)
			continue
		}

		saved := true
		for _, att := range attachments {
			l.infof("  uid %d: attachment %q (%d bytes)", msg.UID, att.Filename, len(att.Data))
			if !noop {
				if err := uploader.Upload(att.Filename, att.Data); err != nil {
					log.Printf("  uid %d: upload %q: %v", msg.UID, att.Filename, err)
					saved = false
				}
			}
		}

		if saved {
			processed = append(processed, msg.UID)
		}
	}

	if noop || len(processed) == 0 {
		return nil
	}

	if deleteAfter {
		if err := fetcher.DeleteMessages(folder, processed); err != nil {
			return fmt.Errorf("delete messages: %w", err)
		}
	} else {
		if err := fetcher.MarkSeen(folder, processed); err != nil {
			return fmt.Errorf("mark seen: %w", err)
		}
	}

	return nil
}

func extractAttachments(rawMsg []byte) ([]Attachment, error) {
	mr, err := mail.CreateReader(bytes.NewReader(rawMsg))
	if err != nil {
		return nil, fmt.Errorf("parse message: %w", err)
	}

	var attachments []Attachment
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("next part: %w", err)
		}

		ah, ok := p.Header.(*mail.AttachmentHeader)
		if !ok {
			continue
		}

		filename, err := ah.Filename()
		if err != nil || filename == "" {
			continue
		}
		if decoded, err := new(mime.WordDecoder).DecodeHeader(filename); err == nil {
			filename = decoded
		}

		data, err := io.ReadAll(p.Body)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", filename, err)
		}

		attachments = append(attachments, Attachment{Filename: filename, Data: data})
	}

	return attachments, nil
}
