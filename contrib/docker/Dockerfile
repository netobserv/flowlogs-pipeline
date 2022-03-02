FROM docker.io/library/golang:1.17 as builder

WORKDIR /app

# Copy source code
COPY go.mod .
COPY go.sum .

COPY cmd/ cmd/
COPY pkg/ pkg/
COPY .bingo/ .bingo/
COPY .git/ .git/
COPY Makefile Makefile

# Download modules
RUN go mod download
RUN go mod vendor

RUN make build_code

# final stage
FROM ubuntu
RUN apt-get update -y
RUN apt-get install -y netbase
RUN apt-get install -y curl
RUN apt-get install -y net-tools
COPY --from=builder /app/flowlogs-pipeline /app/

# expose ports
EXPOSE 2055

ENTRYPOINT ["/app/flowlogs-pipeline"]