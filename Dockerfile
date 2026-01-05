# Build stage
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/ollama-auto-ctx ./cmd/ollama-auto-ctx

# Runtime stage
FROM alpine:3.20
RUN adduser -D -H -u 10001 app
USER app
COPY --from=build /out/ollama-auto-ctx /usr/local/bin/ollama-auto-ctx
EXPOSE 11435
ENTRYPOINT ["/usr/local/bin/ollama-auto-ctx"]
