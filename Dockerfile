# Start from the official Go image to build our application
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25 AS builder

ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with version info
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on \
    go build -ldflags="-s -w -X main.Version=${VERSION}" -o rift-server ./cmd/server

# Start a new stage from scratch
FROM gcr.io/distroless/static:nonroot

ENV SSH_ADDR=:2222
ENV HTTP_ADDR=:8080
ENV BASE_DOMAIN=localhost
ENV LOG_LEVEL=info
ENV IDLE_TIMEOUT=300s
ENV MAX_TIMEOUT=50m
ENV CLEANUP_INTERVAL=5m
ENV SUBDOMAIN_LENGTH=8

USER nonroot

WORKDIR /

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/rift-server .

# Expose ports to the outside world
EXPOSE ${HTTP_ADDR}
EXPOSE ${SSH_ADDR}

# Command to run the executable
ENTRYPOINT ["./rift-server"]
