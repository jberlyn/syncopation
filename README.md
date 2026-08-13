# Joplin Sync Server

A custom, lightweight, 100% compatible Joplin Sync Server written in Go.

## Features
- Complete implementation of the Joplin Server API protocol.
- Sync locks and concurrency control.
- Support for End-to-End Encryption (E2EE) notes.
- Batch uploading and directory traversal fallbacks.

## Running with Docker (Recommended)

1. **Start the server:**
   ```bash
   docker compose up -d
   ```

2. **Seed an admin user:**
   You must create at least one user account to authenticate your Joplin clients. Run this command while the container is running:
   ```bash
   docker compose exec joplin-server ./joplin-sync-server -seed -email your_email@example.com -password your_secure_password
   ```

3. **Configure Joplin Client:**
   - Go to **Options > Synchronization**.
   - Set **Sync target** to **Joplin Server**.
   - **Joplin Server URL**: The URL pointing to your server (e.g. `http://localhost:8080`).
   - **Joplin Server email**: The email you used to seed the user.
   - **Joplin Server password**: The password you used to seed the user.
   - Click "Check synchronization configuration" to verify.

## Backup and Restoration
All metadata and item content (BLOBs) are stored inside the SQLite database located at `/app/data/joplin.sqlite3` inside the container. This directory is mounted to the Docker named volume `joplin-data`.

To back up your data, simply copy the `joplin.sqlite3` file out of the docker volume or use standard docker volume backup procedures.

## Building Locally
Ensure you have Go 1.21+ installed and CGO enabled (required for SQLite).
```bash
go build -o joplin-sync-server .
./joplin-sync-server
```
