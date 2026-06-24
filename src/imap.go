package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type imapClient struct {
	c *imapclient.Client
}

func newIMAPClient(mb Mailbox, l *logger) (MailFetcher, error) {
	addr := net.JoinHostPort(mb.Host, strconv.Itoa(mb.Port))

	opts := &imapclient.Options{}
	if mb.TLS {
		opts.TLSConfig = &tls.Config{ServerName: mb.Host}
	}
	if l != nil && l.debug {
		opts.DebugWriter = log.Writer()
	}

	var (
		c   *imapclient.Client
		err error
	)
	if mb.TLS {
		c, err = imapclient.DialTLS(addr, opts)
	} else {
		c, err = imapclient.DialInsecure(addr, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	if err := imapAuth(c, mb); err != nil {
		c.Close()
		return nil, err
	}

	return &imapClient{c: c}, nil
}

func imapAuth(c *imapclient.Client, mb Mailbox) error {
	if mb.Auth == "oauth2" {
		token, err := gmailAccessToken(mb.OAuth2, mb.Username)
		if err != nil {
			return fmt.Errorf("oauth2: %w", err)
		}
		saslClient := sasl.NewOAuthBearerClient(&sasl.OAuthBearerOptions{
			Username: mb.Username,
			Token:    token,
		})
		return c.Authenticate(saslClient)
	}
	pw, err := mb.Password()
	if err != nil {
		return fmt.Errorf("get password: %w", err)
	}
	return c.Login(mb.Username, pw).Wait()
}

func gmailAccessToken(cfg *OAuth2Config, username string) (string, error) {
	clientSecret, err := getSecret(cfg.ClientSecretEnv, "fetchbox:oauth2:secret", username)
	if err != nil {
		return "", fmt.Errorf("client secret: %w", err)
	}
	refreshToken, err := getSecret(cfg.RefreshTokenEnv, "fetchbox:oauth2:token", username)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://mail.google.com/"},
	}
	ts := oauthCfg.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})
	t, err := ts.Token()
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

func (ic *imapClient) Fetch(folder string, unseenOnly bool) ([]RawMessage, error) {
	if _, err := ic.c.Select(folder, nil).Wait(); err != nil {
		return nil, fmt.Errorf("select %q: %w", folder, err)
	}

	criteria := &imap.SearchCriteria{}
	if unseenOnly {
		criteria.NotFlag = []imap.Flag{imap.FlagSeen}
	}
	searchData, err := ic.c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("uid search: %w", err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	section := &imap.FetchItemBodySection{
		Specifier: imap.PartSpecifierNone,
	}
	fetchCmd := ic.c.Fetch(imap.UIDSetNum(uids...), &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{section},
	})

	var msgs []RawMessage
	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		var (
			uid     imap.UID
			rawBody []byte
		)
		for {
			item := msg.Next()
			if item == nil {
				break
			}
			switch it := item.(type) {
			case imapclient.FetchItemDataUID:
				uid = it.UID
			case imapclient.FetchItemDataBodySection:
				if it.Literal != nil {
					rawBody, err = io.ReadAll(it.Literal)
					if err != nil {
						return nil, fmt.Errorf("read literal: %w", err)
					}
				}
			}
		}

		if uid != 0 && rawBody != nil {
			msgs = append(msgs, RawMessage{UID: uint32(uid), Data: rawBody})
		}
	}

	if err := fetchCmd.Close(); err != nil {
		return nil, fmt.Errorf("fetch close: %w", err)
	}

	return msgs, nil
}

func (ic *imapClient) DeleteMessages(folder string, uids []uint32) error {
	if len(uids) == 0 {
		return nil
	}
	imapUIDs := make([]imap.UID, len(uids))
	for i, uid := range uids {
		imapUIDs[i] = imap.UID(uid)
	}
	uidSet := imap.UIDSetNum(imapUIDs...)

	if err := ic.c.Store(uidSet, &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagDeleted},
	}, nil).Close(); err != nil {
		return fmt.Errorf("mark deleted: %w", err)
	}

	if _, err := ic.c.Expunge().Collect(); err != nil {
		return fmt.Errorf("expunge: %w", err)
	}
	return nil
}

func (ic *imapClient) MarkSeen(folder string, uids []uint32) error {
	if len(uids) == 0 {
		return nil
	}

	imapUIDs := make([]imap.UID, len(uids))
	for i, uid := range uids {
		imapUIDs[i] = imap.UID(uid)
	}
	uidSet := imap.UIDSetNum(imapUIDs...)

	return ic.c.Store(uidSet, &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagSeen},
	}, nil).Close()
}

func (ic *imapClient) Close() error {
	return ic.c.Logout().Wait()
}

// listFolders connects to mb and prints all mailbox names to stdout.
func listFolders(mb Mailbox) error {
	client, err := newIMAPClient(mb, nil)
	if err != nil {
		return err
	}
	defer client.Close()

	raw := client.(*imapClient).c
	cmd := raw.List("", "*", nil)
	defer cmd.Close()
	for {
		data := cmd.Next()
		if data == nil {
			break
		}
		fmt.Println(data.Mailbox)
	}
	return nil
}
