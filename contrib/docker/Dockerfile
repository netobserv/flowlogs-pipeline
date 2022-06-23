FROM docker.io/library/golang:1.17 as builder

WORKDIR /app

# Copy source code
COPY go.mod .
COPY go.sum .
COPY .bingo/ .bingo/

# Download modules
RUN go mod download
RUN go mod download -modfile=.bingo/golangci-lint.mod

COPY . ./
RUN rm -rf bin
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
