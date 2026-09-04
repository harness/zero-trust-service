# Multi-stage Dockerfile for Zero Trust Service.
# Builds the Go binary from source and packages it into a minimal image.
#
# Build (from repo root):
#   docker build -t <your-repo>/zero-trust-service:latest -f examples/zts/docker/Dockerfile .
#
# Run (minimal — env vars only):
#   docker run -p 4210:4210 -p 8898:8898 \
#     -e ZTS_ALLOWED_ACCOUNTS='["myAccountId"]' \
#     -e GITHUB_TOKEN=ghp_xxx \
#     <your-repo>/zero-trust-service:latest
#
# Run (with custom config + mappings):
#   docker run -p 4210:4210 -p 8898:8898 \
#     -v $(pwd)/config.yaml:/app/config.yaml \
#     -v $(pwd)/mappings.yaml:/app/mappings.yaml \
#     <your-repo>/zero-trust-service:latest

FROM golang:1.24-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/zero-trust-service ./examples/zts

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/zero-trust-service /bin/zero-trust-service
COPY examples/zts/config.yaml /app/config.yaml

EXPOSE 4210 8898

ENTRYPOINT ["/bin/zero-trust-service"]
CMD ["-config", "/app/config.yaml"]
