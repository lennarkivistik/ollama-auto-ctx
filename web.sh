# Build frontend
cd frontend && bun install && bun run build && cd ..

# Copy to embed location
rm -rf web/dist && cp -r frontend/dist web/dist

# Build Go
#go build -o ollama-auto-ctx ./cmd/ollama-auto-ctx