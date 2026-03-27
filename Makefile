.PHONY: setup dev-server dev-worker build dashboard docker-up docker-down

# first time setup  downloads all go deps and node modules
setup:
	go mod tidy
	cd dashboard && npm install

# run the server locally (needs postgres + redis running)
dev-server:
	go run ./cmd/server

# run a worker locally
dev-worker:
	go run ./cmd/worker

# build both binaries
build:
	go build -o bin/server ./cmd/server
	go build -o bin/worker ./cmd/worker

# build the dashboard
dashboard:
	cd dashboard && npm run build

# start everything with docker compose
docker-up:
	docker compose up --build

docker-down:
	docker compose down

# spin up 3 workers
scale-workers:
	docker compose up --scale worker=3 -d
