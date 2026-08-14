# Syncopation

[![Build Status](https://img.shields.io/github/actions/workflow/status/jberlyn/syncopation/badges.yml?branch=main&label=build)](https://github.com/jberlyn/syncopation/actions/workflows/badges.yml)
[![Coverage Status](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fjberlyn%2Fsyncopation%2Fbadges%2Fcoverage.json)](https://github.com/jberlyn/syncopation/actions)
[![Version](https://img.shields.io/github/v/tag/jberlyn/syncopation?label=version)](https://github.com/jberlyn/syncopation/tags)
[![Docker Pulls](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fjberlyn%2Fsyncopation%2Fbadges%2Fdocker_pulls.json)](https://github.com/jberlyn/syncopation/pkgs/container/syncopation)

A custom, lightweight, 100% compatible Joplin Sync Server written in Go.

### Why the name?
The official [Joplin](https://joplinapp.org/help/faq/#why-is-it-named-joplin) app is named in honor of the famous composer and pianist, Scott Joplin. This custom server is named **Syncopation** as a play on words: it **syncs** your notes, and *syncopation* is a core musical concept that drives the ragtime rhythm Scott Joplin is known for.

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

## Self-Hosting Guide (Docker)

This is the recommended way to run Syncopation on your home server, NAS, or VPS.

### 1. Docker Compose Setup

Create a `docker-compose.yml` file:

```yaml
services:
  syncopation:
    image: ghcr.io/jberlyn/syncopation:latest
    container_name: syncopation
    restart: unless-stopped
    ports:
      - "22300:22300"
    volumes:
      - syncopation-data:/app/data

volumes:
  syncopation-data:
```

### 2. Volumes & Bind Mounts

The server stores both the SQLite database (`syncopation.sqlite`) and all physical note attachments inside the `/app/data` directory within the container. 
It is crucial that you persist this directory.
- **Named Volume (Example above)**: Best for easy management by Docker.
- **Bind Mount**: If you prefer storing the data in a specific folder on your host (e.g., `- ./my-syncopation-data:/app/data`), ensure the container has write permissions to that directory.

### 3. Running & Initial Setup

Start the server:
```bash
docker compose up -d
```

Visit the Admin UI at `http://localhost:22300/` in your browser. 
Since this is a fresh install, you will be presented with a short wizard to create your initial administrator account. All accounts can be used to sync with Joplin, but only admin accounts can be used to login to the Admin UI.

### 4. Reverse Proxy Recommendations

It is highly recommended to place Syncopation behind a reverse proxy (like Caddy, Nginx, or Traefik) to provide HTTPS/TLS.

**Important**: Ensure your reverse proxy is configured to allow large request bodies (Joplin can upload large attachments) and passes standard headers.
- **Nginx**: Add `client_max_body_size 100M;` to your server block.
- **Caddy**: Automatically handles large bodies and SSL. A simple Caddyfile:
  ```caddyfile
  syncopation.yourdomain.com {
      reverse_proxy localhost:22300
  }
  ```

### 5. Connecting your Joplin Client

1. Open Joplin and go to **Options > Synchronization**.
2. Set **Sync target** to **Joplin Server**.
3. **Joplin Server URL**: Your server's URL (e.g., `https://joplin.yourdomain.com` or `http://your-server-ip:22300`).
4. **Joplin Server email**: The email of the account you created.
5. **Joplin Server password**: Your password.
6. Click "Check synchronization configuration".

## Local Development

### Prerequisites
- Go 1.25+
- `sqlc` (for generating DB queries)

*Note: Syncopation uses the pure-Go `modernc.org/sqlite` driver, so CGO is **not** required. This makes the project extremely easy to build and cross-compile!*

### Building and Running
```bash
go build -o syncopation .
./syncopation
```

### Hot Reloading (Recommended)
For local development, the Go ecosystem standard [Air](https://github.com/air-verse/air) is used for hot reloading. It automatically recompiles the binary and restarts the server whenever you modify `.go` or `.html` template files.

1. Install `air` globally:
```bash
go install github.com/air-verse/air@latest
```
2. Start the development server (make sure `~/go/bin` is in your `$PATH`):
```bash
air
```

### Testing
Syncopation uses Go's standard `testing` package.
- **Unit & Integration Tests**: Run with `go test ./...`
- **E2E Tests**: Use `net/http/httptest` via `go test ./api/...`

## AI Disclosure

This project was built with the assistance of AI tooling such as Antigravity following a Research, Plan, Implement strategy.

## License

This software is licensed under the **PolyForm Noncommercial License 1.0.0**.
You may use, modify, and distribute this software for any non-commercial purpose (personal use, hobbies, research, etc.).

If you wish to use this software for commercial purposes (including offering it as a paid service), you must obtain explicit permission or a commercial license from the author. For full license terms, please see the [LICENSE](LICENSE) file.
