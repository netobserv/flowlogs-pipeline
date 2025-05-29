ARG TARGETARCH
FROM docker.io/library/golang:1.24 as builder

ARG TARGETARCH=amd64
ARG LDFLAGS
WORKDIR /app

# Copy source code
COPY go.mod .
COPY go.sum .
COPY vendor/ vendor/
COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build -ldflags "$LDFLAGS" -mod vendor -o flowlogs-pipeline cmd/flowlogs-pipeline/main.go

# final stage
FROM --platform=linux/$TARGETARCH registry.access.redhat.com/ubi9/ubi-minimal:9.6-1747218906

COPY --from=builder /app/flowlogs-pipeline /app/

ENTRYPOINT ["/app/flowlogs-pipeline"]
