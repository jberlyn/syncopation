# Syncopation (Joplin Sync Server)

A custom, lightweight, 100% compatible Joplin Sync Server written in Go.

## Features
- **100% Joplin Client Compatibility**: Seamlessly works with official Joplin Desktop, Mobile, and CLI clients.
- **End-to-End Encryption (E2EE)**: Fully supports transparent syncing of encrypted notes.
- **Efficient Sync Engine**: Handles delta syncs, sync locks, concurrency control, and batch operations.
- **Lightweight & Fast**: Built in Go, utilizing a single static binary and minimal memory footprint.
- **Admin Web UI**: Built-in administration panel using HTMX.
- **Simple Deployment**: Single Docker container with SQLite and Local Disk storage. No external database required.

## Tech Stack
- **Backend**: Go (Standard Library `net/http`)
- **Database**: SQLite (via `sqlc`)
- **Storage**: Local Filesystem
- **Admin UI**: Go `html/template` + HTMX

---

## 🏡 Self-Hosting Guide (Docker)

This is the recommended way to run Syncopation on your home server, NAS, or VPS.

### 1. Docker Compose Setup

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  syncopation:
    image: ghcr.io/jberlyn/syncopation:latest
    container_name: syncopation
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - syncopation-data:/app/data
    environment:
      - PORT=8080

volumes:
  syncopation-data:
```

### 2. Volumes & Bind Mounts

The server stores both the SQLite database (`syncopation.sqlite`) and all physical note attachments inside the `/app/data` directory within the container. 
It is crucial that you persist this directory.
- **Named Volume (Example above)**: Best for easy management by Docker.
- **Bind Mount**: If you prefer storing the data in a specific folder on your host (e.g., `- ./my-joplin-data:/app/data`), ensure the container has write permissions to that directory.

### 3. Running & Initial Setup

Start the server:
```bash
docker compose up -d
```

Visit the Admin UI at `http://localhost:8080/admin` in your browser. 
Since this is a fresh install, you will be presented with a **Zero-User Onboarding Flow** to create your initial administrator account.

### 4. Reverse Proxy Recommendations

It is highly recommended to place Syncopation behind a reverse proxy (like Caddy, Nginx, or Traefik) to provide HTTPS/TLS.

**Important**: Ensure your reverse proxy is configured to allow large request bodies (Joplin can upload large attachments) and passes standard headers.
- **Nginx**: Add `client_max_body_size 100M;` to your server block.
- **Caddy**: Automatically handles large bodies and SSL. A simple Caddyfile:
  ```caddyfile
  joplin.yourdomain.com {
      reverse_proxy localhost:8080
  }
  ```

### 5. Connecting your Joplin Client

1. Open Joplin and go to **Options > Synchronization**.
2. Set **Sync target** to **Joplin Server**.
3. **Joplin Server URL**: Your server's URL (e.g., `https://joplin.yourdomain.com` or `http://your-server-ip:8080`).
4. **Joplin Server email**: The email of the account you created.
5. **Joplin Server password**: Your password.
6. Click "Check synchronization configuration".

### 6. Backup Strategy

To back up your data, you must back up both the database and the files. Because they are both located in the `/app/data` directory, a simple backup script just needs to archive that folder.

*Note: For the safest database backup, it's recommended to stop the container first, or use the `sqlite3` CLI tool to create a backup of the DB while running.*

```bash
docker compose stop syncopation
tar -czvf joplin-backup-$(date +%F).tar.gz /path/to/your/data/dir
docker compose start syncopation
```

---

## 🛠️ Local Development

### Prerequisites
- Go 1.21+
- CGO enabled (required for SQLite `mattn/go-sqlite3` driver)
- `sqlc` (for generating DB queries)

### Building and Running
```bash
go build -o syncopation .
./syncopation
```

### Testing
We use Go's standard `testing` package.
- **Unit & Integration Tests**: Run with `go test ./...`
- **E2E Tests**: Use `net/http/httptest` via `go test ./api/...`
