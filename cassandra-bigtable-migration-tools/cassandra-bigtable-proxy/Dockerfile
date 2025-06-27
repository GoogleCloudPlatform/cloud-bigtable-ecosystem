FROM golang:1.24.0 as builder

# Disable cgo to remove gcc dependency
ENV CGO_ENABLED=0

WORKDIR /go/src/cassandra-to-bigtable-proxy

# Grab the dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy in source
COPY . ./

# Build and install binary
RUN go install github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/cassandra-to-bigtable-proxy .

# a new clean image with just the binary
FROM alpine:3.14
RUN apk add --no-cache ca-certificates

EXPOSE 9042 9043 9044 9045 9046 9047 9048 9049 9050 9051

# Copy in the binary
COPY --from=builder /go/bin/cassandra-to-bigtable-proxy  .

COPY config.yaml /

ENTRYPOINT ["/cassandra-to-bigtable-proxy", "--rpc-address", "10.5.0.41"]
