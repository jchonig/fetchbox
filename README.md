# docker-fetchbox

FetchBox is a Go daemon that polls IMAP mailboxes — including Gmail via IMAP with OAuth2 — extracts attachments, and saves them to a WebDAV server. It runs as an s6-supervised service inside a [linuxserver.io](https://www.linuxserver.io/) Alpine container.

Example deployment: a Proton Mail Bridge container on the same Docker network provides an IMAP endpoint for ProtonMail; FetchBox polls it alongside Gmail (IMAP + OAuth2) and any other standard IMAP server.

---

## Quick start

```bash
git clone https://github.com/jchonig/docker-fetchbox.git
cd docker-fetchbox
cp config/fetchbox.yml.example config/fetchbox.yml   # edit as needed
cp .env.example .env                                  # fill in secrets
docker compose up -d
```

### One-time Proton Mail Bridge login

```bash
docker compose exec bridge /protonmail/protonmail-bridge --cli
# login → quit — auth persists in the bridge-data volume
docker compose restart fetchbox
```

---

## Configuration

FetchBox reads `/config/fetchbox.yml` (mount your local `config/` directory as `/config:ro`).

```yaml
interval: 5m          # polling interval (any Go duration string)

storage:
  nextcloud:           # arbitrary name, referenced by folders
    type: webdav
    url: webdavs://you@nextcloud.example.com/remote.php/webdav/FetchBox/
    password_env: WEBDAV_PASSWORD

mailboxes:
  - name: ProtonMail
    host: bridge        # Docker service name on the internal network
    port: 1143
    tls: false
    username: you@proton.me
    password_env: PROTON_PASSWORD
    folders:
      - name: INBOX
        storage: nextcloud
        path: /proton/

  - name: Gmail
    host: imap.gmail.com
    port: 993
    tls: true
    username: you@gmail.com
    auth: oauth2
    oauth2:
      client_id_env: GMAIL_CLIENT_ID
      client_secret_env: GMAIL_CLIENT_SECRET
      refresh_token_env: GMAIL_REFRESH_TOKEN
    folders:
      - name: INBOX
        storage: nextcloud
        path: /gmail/
```

### Mailbox fields

| Field | Description |
|---|---|
| `name` | Display name used in logs |
| `host` | IMAP server hostname |
| `port` | IMAP server port |
| `tls` | `true` for direct TLS (e.g. port 993); `false` for plain |
| `starttls` | `true` to upgrade a plain connection with STARTTLS |
| `username` | IMAP login username |
| `password_env` | Name of the environment variable containing the IMAP password |
| `auth` | `plain` (default) or `oauth2` |
| `oauth2` | OAuth2 credentials block (Gmail); see below |
| `folders` | List of folders to watch |

### OAuth2 fields (Gmail)

| Field | Description |
|---|---|
| `client_id_env` | Env var with the Google OAuth2 client ID |
| `client_secret_env` | Env var with the Google OAuth2 client secret |
| `refresh_token_env` | Env var with the offline refresh token |

### Storage fields

| Field | Description |
|---|---|
| `type` | `webdav` (the only type currently supported) |
| `url` | WebDAV base URL using `webdavs://` (TLS) or `webdav://` scheme; embed the username in the URL, e.g. `webdavs://user@host/path/` |
| `password_env` | Name of the environment variable containing the WebDAV password |

### Folder fields

| Field | Description |
|---|---|
| `name` | IMAP folder name (e.g. `INBOX`) |
| `storage` | Name of a storage entry defined in the top-level `storage:` map |
| `path` | Path within the storage base URL where attachments are placed |

---

## Environment variables

Secrets are passed via environment variables referenced by name in the config file. Create a `.env` file (never committed — it is in `.gitignore`):

```
PROTON_PASSWORD=
WEBDAV_PASSWORD=
GMAIL_CLIENT_ID=
GMAIL_CLIENT_SECRET=
GMAIL_REFRESH_TOKEN=
```

---

## CLI flags

When running outside of the container (for debugging):

```
fetchbox [flags]
  --config path        path to config file (default /config/fetchbox.yml)
  --daemon             run continuously at the configured interval
  --list-folders       list available IMAP folders for each mailbox and exit
  -v                   verbose logging
  -d                   debug logging
  -n                   dry run — fetch but do not upload or mark messages seen
```

---

## Behaviour

On each poll cycle, for every configured mailbox and folder:

1. Connect to the IMAP server and authenticate.
2. UID-search for unseen (`\Seen` flag absent) messages.
3. Fetch the full RFC 5322 message for each match.
4. Extract MIME attachments.
5. HTTP PUT each attachment to the configured WebDAV destination.
6. Mark each successfully processed message as `\Seen`.

Messages with no attachments are marked seen immediately so they are not re-examined on the next cycle.

---

## Docker Compose

```yaml
services:
  bridge:
    image: shenxn/protonmail-bridge:latest
    restart: unless-stopped
    volumes:
      - bridge-data:/root       # persists bridge auth across restarts

  fetchbox:
    image: ghcr.io/jchonig/docker-fetchbox:latest
    restart: unless-stopped
    depends_on:
      - bridge
    volumes:
      - ./config:/config:ro
    env_file:
      - .env

volumes:
  bridge-data:
```

The Proton Mail Bridge stores its session tokens in `~/.config/protonmail/bridge-v3/` (mounted at `/root` above). Auth persists across restarts; only an initial interactive login is needed.

---

## Updates

```bash
docker compose pull && docker compose up -d
```

CI pushes a new `latest` image on every merge to `main`.

---

## Development

All build and test steps run inside Docker — nothing needs to be installed locally.

```bash
make build          # compile check (golang:1.26-alpine)
make lint           # go vet + gofmt check
make test           # go test -race ./... (golang:1.26)
make docker-build   # build the container image
make docker-push    # push to ghcr.io
```

Go source lives in `src/`. Container overlay files (s6 service, etc.) live in `root/`.

### Git hooks

Pre-commit hooks run `make lint` and `make test` before every commit. Enable them after cloning:

```bash
make hooks
```

This sets `core.hooksPath = .githooks` in the local git config. The hook scripts are tracked in `.githooks/` so they stay in sync with the repo.

### Running against a live mailbox

```bash
docker run --rm \
  -v "$PWD/config:/config:ro" \
  --env-file .env \
  ghcr.io/jchonig/docker-fetchbox:latest \
  fetchbox --list-folders
```
