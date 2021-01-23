ARG GOLANG_VERSION

FROM golang:$GOLANG_VERSION as builder

# git used for app version fetch
RUN apk add --no-cache git

WORKDIR /opt/app

# Cached layer
COPY ./go.mod ./go.sum ./
RUN go mod download
RUN go get github.com/go-bindata/go-bindata/...
RUN go get github.com/elazarl/go-bindata-assetfs/...

# Sources dependent layer
COPY ./ ./
RUN go generate ./cmd/webtail/...
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=`git describe --tags --always`" -a ./cmd/webtail

FROM scratch

MAINTAINER Alexey Kovrizhkin <lekovr+dopos@gmail.com>

VOLUME /data
WORKDIR /
COPY --from=builder /opt/app/webtail .
EXPOSE 8080
ENTRYPOINT ["/webtail"]
