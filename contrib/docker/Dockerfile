FROM registry.access.redhat.com/ubi8/go-toolset:1.16.12-10 as builder
ARG BUILD_VERSION="unknown"

WORKDIR /opt/app-root

# TEMPORARY STEPS UNTIL ubi8 releases a go1.17 image
RUN wget --no-check-certificate https://go.dev/dl/go1.17.6.linux-amd64.tar.gz && tar -xzf go1.17.6.linux-amd64.tar.gz
ENV GOROOT /opt/app-root/go
ENV PATH $GOROOT/bin:$PATH
# END OF LINES TO REMOVE

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
FROM registry.access.redhat.com/ubi8/ubi-minimal
USER root
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
