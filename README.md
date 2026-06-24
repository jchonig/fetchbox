# fetchbox

FetchBox is a Go daemon that polls IMAP mailboxes — including Gmail via IMAP with OAuth2 — extracts attachments, and saves them to a WebDAV server (e.g. Nextcloud).

Two deployment modes:

- **macOS native binary** — installs via Homebrew; credentials stored in the macOS Keychain; integrates with the [Proton Mail Bridge](https://proton.me/mail/bridge) desktop app running on localhost.
- **Docker container** — runs as an s6-supervised service inside a [linuxserver.io](https://www.linuxserver.io/) Alpine container; secrets passed via environment variables; works alongside a Proton Mail Bridge container on the same Docker network.

---

## macOS Quick Start

### Install

```bash
brew install jchonig/tap/fetchbox
```

Or download a release tarball from the [GitHub Releases page](https://github.com/jchonig/docker-fetchbox/releases) and place the binary in your `PATH`.

### Configure

```bash
mkdir -p ~/.config
cp /path/to/fetchbox.yml.example ~/.config/fetchbox.yml
# edit ~/.config/fetchbox.yml
```

### First run — populate Keychain

Run interactively once so FetchBox can prompt for and store each secret:

```bash
fetchbox --list-folders
```

On first use, FetchBox will prompt for each password/secret not already in the Keychain and store it under the `fetchbox` service. Subsequent runs (and the background daemon) read from the Keychain without prompting.

### Install as a launchd service

```bash
fetchbox --install
```

This writes `~/Library/LaunchAgents/net.honig.fetchbox.plist`, bootstraps the service immediately, and logs to `~/Library/Logs/fetchbox.log`.

```bash
fetchbox --uninstall    # remove the service
```

---

## Docker Quick Start

```bash
git clone https://github.com/jchonig/docker-fetchbox.git
cd docker-fetchbox
cp config/fetchbox.yml.example config/fetchbox.yml   # edit as needed
cp .env.example .env                                  # fill in secrets
docker compose up -d
```

### One-time Proton Mail Bridge login (Docker)

```bash
# Open a shell in the bridge container, stop the background bridge instance,
# then run the interactive CLI to authenticate
docker compose exec bridge /bin/bash
pkill bridge
/usr/bin/bridge --cli
# Inside the CLI:
#   >>> login
#   >>> exit
exit

# Restart so the bridge picks up the saved credentials
docker compose restart bridge
```

---

## Configuration

The default config path is `~/.config/fetchbox.yml`. Override with `--config`.

When running in Docker, mount your local `config/` directory as `/config:ro`; the container reads `/config/fetchbox.yml`.

```yaml
interval: 5m          # polling interval (any Go duration string)

storage:
  nextcloud:           # arbitrary name, referenced by folders
    type: webdav
    url: webdavs://you@nextcloud.example.com/remote.php/webdav/FetchBox/
    password_env: WEBDAV_PASSWORD   # optional on macOS — Keychain used if unset

mailboxes:
  - name: ProtonMail
    host: localhost     # Proton Mail Bridge desktop app (macOS)
    # host: bridge      # Docker service name on the internal network
    port: 1143
    tls: false
    username: you@proton.me
    password_env: PROTON_PASSWORD   # optional on macOS — Keychain used if unset
    folders:
      - name: INBOX
        storage: nextcloud
        path: /proton/
      - name: Labels/RecordKeeper
        storage: nextcloud
        path: /recordkeeper/
        delete_after: true   # expunge after upload

  - name: Gmail
    host: imap.gmail.com
    port: 993
    tls: true
    username: you@gmail.com
    auth: oauth2
    oauth2:
      client_id: your-client-id.apps.googleusercontent.com
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
| `password_env` | Env var containing the IMAP password. On macOS, falls back to the Keychain if unset or empty. |
| `auth` | `plain` (default) or `oauth2` |
| `oauth2` | OAuth2 credentials block (Gmail); see below |
| `folders` | List of folders to watch |

### OAuth2 fields (Gmail)

| Field | Description |
|---|---|
| `client_id` | Google OAuth2 client ID (not a secret; stored directly in config) |
| `client_secret_env` | Env var with the Google OAuth2 client secret. On macOS, falls back to Keychain. |
| `refresh_token_env` | Env var with the offline refresh token. On macOS, falls back to Keychain. |

### Storage fields

| Field | Description |
|---|---|
| `type` | `webdav` (the only type currently supported) |
| `url` | WebDAV base URL using `webdavs://` (TLS) or `webdav://` scheme; embed the username in the URL, e.g. `webdavs://user@host/path/` |
| `password_env` | Env var containing the WebDAV password. On macOS, falls back to Keychain. |

### Folder fields

| Field | Description |
|---|---|
| `name` | IMAP folder name (e.g. `INBOX`) |
| `storage` | Name of a storage entry defined in the top-level `storage:` map |
| `path` | Path within the storage base URL where attachments are placed |
| `delete_after` | If `true`, fetch **all** messages (not just unseen) and expunge them after upload. Default `false` (mark seen). Use for label-based processing queues where messages should be removed after saving. |

---

## Secrets

### macOS

Secrets are looked up in this order:

1. The environment variable named by `*_env` in the config (if set and non-empty).
2. The macOS Keychain (`security find-generic-password -s fetchbox -a <account>`).
3. Interactive prompt on a TTY — the entered value is stored in the Keychain for future use.

Run `fetchbox --list-folders` once interactively to seed all secrets into the Keychain before starting the background daemon.

### Docker / Linux

Secrets are passed via environment variables referenced by name in the config. Create a `.env` file (never committed — it is in `.gitignore`):

```
PROTON_PASSWORD=
WEBDAV_PASSWORD=
GMAIL_CLIENT_SECRET=
GMAIL_REFRESH_TOKEN=
```

---

## CLI flags

```
fetchbox [flags]
  --config path        path to config file (default ~/.config/fetchbox.yml)
  --daemon             run continuously at the configured interval
  --list-folders       list available IMAP folders for each mailbox and exit
  --install            install launchd service and exit (macOS only)
  --uninstall          remove launchd service and exit (macOS only)
  -v                   verbose logging (connect/login/folder messages)
  -d                   debug logging (includes IMAP protocol trace)
  -n                   dry run — fetch but do not upload, mark seen, or delete
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
    image: ghcr.io/videocurio/proton-mail-bridge:latest
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

The [VideoCurio Proton Mail Bridge](https://github.com/VideoCurio/ProtonMailBridgeDocker) image stores credentials under `/root` — the named volume above persists them across restarts. Only an initial interactive login is needed (see Docker Quick Start above). The bridge listens on port 143 for IMAP inside the Docker network.

---

## Updates

### macOS

```bash
brew upgrade fetchbox
```

### Docker

```bash
docker compose pull && docker compose up -d
```

CI pushes a new `latest` image on every merge to `main`.

---

## Development

All build and test steps run inside Docker — nothing needs to be installed locally.

```bash
make build                # compile check (golang:1.26-alpine)
make lint                 # go vet + gofmt check
make test                 # go test -race ./... (golang:1.26)
make build-darwin-arm64   # cross-compile macOS arm64 → dist/fetchbox-darwin-arm64.tar.gz
make build-darwin-amd64   # cross-compile macOS amd64 → dist/fetchbox-darwin-amd64.tar.gz
make release              # both darwin tarballs
make docker-build         # build the container image
make docker-push          # push to ghcr.io
```

Go source lives in `src/`. Container overlay files (s6 service, etc.) live in `root/`.

### Git hooks

Pre-commit hooks run `make lint` and `make test` before every commit. Enable them after cloning:

```bash
make hooks
```

This sets `core.hooksPath = .githooks` in the local git config. The hook scripts are tracked in `.githooks/` so they stay in sync with the repo.

### Running against a live mailbox (Docker)

```bash
docker run --rm \
  -v "$PWD/config:/config:ro" \
  --env-file .env \
  ghcr.io/jchonig/docker-fetchbox:latest \
  fetchbox --list-folders
```
