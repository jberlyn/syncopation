.PHONY: run seed build docker-up docker-down

# Run the server locally using go run
run:
	go run main.go

# Seed the initial admin user locally
seed:
	go run main.go -seed -email admin@localhost -password admin

# Build the server into a binary
build:
	go build -o joplin-sync-server .

# Spin up the docker container
docker-up:
	docker compose up -d --build

# Stop the docker container
docker-down:
	docker compose down
