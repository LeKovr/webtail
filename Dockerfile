ARG GOLANG_VERSION

# FROM golang:$GOLANG_VERSION as builder
FROM ghcr.io/dopos/golang-alpine:1.16.10-alpine3.14 as builder

WORKDIR /opt/app

# Cached layer
COPY ./go.mod ./go.sum ./
RUN  go install github.com/phogolabs/parcello/cmd/parcello@v0.8.2
RUN  go mod tidy

# Sources dependent layer
COPY ./ ./
RUN go generate ./cmd/webtail/...
RUN CGO_ENABLED=0 go test -tags test -covermode=atomic -coverprofile=coverage.out ./...
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=`git describe --tags --always`" -a ./cmd/webtail

FROM scratch

MAINTAINER Alexey Kovrizhkin <lekovr+dopos@gmail.com>

VOLUME /data
WORKDIR /
COPY --from=builder /opt/app/webtail .
EXPOSE 8080
ENTRYPOINT ["/webtail"]
