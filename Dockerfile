ARG GOLANG_VERSION

# FROM golang:$GOLANG_VERSION as builder
FROM ghcr.io/dopos/golang-alpine:v1.16.10-alpine3.14.2 as builder
ARG TARGETARCH

WORKDIR /opt/app

# Cached layer
COPY ./go.mod ./go.sum ./

RUN echo "Build for arch $TARGETARCH"

# Sources dependent layer
COPY ./ ./
RUN CGO_ENABLED=0 go test -tags test -covermode=atomic -coverprofile=coverage.out ./...
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=`git describe --tags --always`" -a ./cmd/webtail

FROM scratch

MAINTAINER Alexey Kovrizhkin <lekovr+dopos@gmail.com>
LABEL org.opencontainers.image.description "Tail [log]files via web[sockets]"

VOLUME /data
WORKDIR /
COPY --from=builder /opt/app/webtail .
EXPOSE 8080
ENTRYPOINT ["/webtail"]
