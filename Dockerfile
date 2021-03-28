FROM golang:1.16 AS builder

WORKDIR /src

# -- Caching modules
COPY go.mod go.sum ./
RUN go mod download

COPY externalscaler/externalscaler.pb.go externalscaler/externalscaler.pb.go
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o external-scaler

FROM alpine:latest

WORKDIR /
COPY --from=builder /src/external-scaler .

ENTRYPOINT ["/external-scaler"]
