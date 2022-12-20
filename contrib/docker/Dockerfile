# TARGETPLATFORM and BUILDPLATFORM are automatically field when specifying an architecture during build but they need to be declared
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.18 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETARCH

WORKDIR /app

# Copy source code
COPY go.mod .
COPY go.sum .
COPY Makefile .
COPY .bingo/ .bingo/
COPY vendor/ vendor/
COPY .git/ .git/
COPY cmd/ cmd/
COPY pkg/ pkg/

RUN git status --porcelain
RUN GOARCH=$TARGETARCH make build_code

# final stage
FROM  --platform=$TARGETPLATFORM registry.access.redhat.com/ubi8/ubi-minimal:8.7

COPY --from=builder /app/flowlogs-pipeline /app/
COPY --from=builder /app/confgenerator /app/

# expose ports
EXPOSE 2055

ENTRYPOINT ["/app/flowlogs-pipeline"]
