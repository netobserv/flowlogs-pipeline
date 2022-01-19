FROM docker.io/library/golang:1.17 as builder

WORKDIR /app

# Copy source code
COPY go.mod .
COPY go.sum .

# Download modules
RUN --mount=type=cache,mode=0755,target=/root/.cache/go-build --mount=type=cache,mode=0755,target=/root/go \
    go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN --mount=type=cache,mode=0755,target=/root/.cache/go-build --mount=type=cache,mode=0755,target=/root/go \
    CGO_ENABLED=0 GOOS=linux GO111MODULE=on GOARCH=amd64 go build ./cmd/flowlogs2metrics

# final stage
FROM ubuntu
RUN apt-get update -y
RUN apt-get install -y netbase
COPY --from=builder /app/flowlogs2metrics /app/

# expose ports
EXPOSE 2055

ENTRYPOINT ["/app/flowlogs2metrics"]