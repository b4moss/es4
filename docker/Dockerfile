# Es4 Server — multi-stage static binary.
# Build from repository root:
#   docker build -f docker/Dockerfile .
#
# Cloud Run sets PORT; the server prefers PORT over ES4_LISTEN_ADDR (default :8080).
# Health probes (no trailing slash):
#   GET /livez
#   GET /readyz

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY packages/go/go.mod packages/go/go.sum ./packages/go/
WORKDIR /src/packages/go
RUN go mod download
COPY packages/go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/es4-server ./cmd/es4-server

# Includes CA certs for HTTPS Recovery (libSQL / Object).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/es4-server /es4-server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/es4-server"]
