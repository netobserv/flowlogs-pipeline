FROM docker.io/library/golang:1.18 as builder

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
RUN make build_code

# final stage
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6
RUN microdnf install -y \
  iputils \
  curl \
  net-tools \
  && microdnf -y clean all  && rm -rf /var/cache

COPY --from=builder /app/flowlogs-pipeline /app/
COPY --from=builder /app/confgenerator /app/

# expose ports
EXPOSE 2055

ENTRYPOINT ["/app/flowlogs-pipeline"]
