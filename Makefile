.PHONY: all build frontend backend run dev dev-frontend dev-backend clean test fmt dist-macos dist-clean

# Full build: frontend then Go binary
all: build

build: frontend backend

# Build frontend
frontend:
	cd frontend && npm install && npm run build

# Build Go binary (requires frontend/build to exist)
backend:
	CGO_ENABLED=1 go build -o fllint .

# Run the built binary
run: build
	./fllint

# --- Development ---

# Run both servers concurrently (trap ensures Go backend is killed when Vite exits)
dev:
	@echo "Starting Go backend on :8420 and Vite dev server on :5173..."
	@echo "Open http://localhost:5173 for development"
	@trap 'kill 0' EXIT; \
	 go run -tags dev . & \
	 cd frontend && npm run dev

# Frontend dev server (Vite with HMR, proxies /api to Go)
dev-frontend:
	cd frontend && npm run dev

# Go backend only (no embedded frontend)
dev-backend:
	go run -tags dev .

# --- Utilities ---

clean:
	rm -f fllint
	rm -rf frontend/build
	rm -rf frontend/.svelte-kit

# Install frontend dependencies only
frontend-deps:
	cd frontend && npm install

test:
	go test ./...

fmt:
	go fmt ./...

# --- Distribution ---

# Build macOS .app distribution folder
dist-macos: build
	@bash packaging/macos/build-app.sh

# Clean distribution artifacts
dist-clean:
	rm -rf dist/
