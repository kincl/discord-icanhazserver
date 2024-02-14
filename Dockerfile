FROM golang:1.21.7 as builder

WORKDIR /go/src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .

RUN go build -o icanhazserver main.go

FROM quay.io/centos/centos:stream9-minimal

COPY --from=builder /go/src/icanhazserver /bin/icanhazserver
