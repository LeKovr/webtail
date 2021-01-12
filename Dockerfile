
# cloud.docker.com does not use ARG, so golang_version must be hardcoded
# ARG golang_version
# FROM golang:$golang_version

FROM golang:1.15.5-alpine3.12 as builder

WORKDIR /opt/app
# Used in `git describe`
RUN apk --update add git

# Cached layer
COPY ./go.mod ./go.sum ./
RUN go mod download
RUN go get github.com/go-bindata/go-bindata/...
RUN go get github.com/elazarl/go-bindata-assetfs/...

# Sources dependent layer
COPY ./ ./
RUN go generate ./cmd/webtail/...
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=`git describe --tags --always`" -a ./cmd/webtail/

FROM scratch

VOLUME /data

WORKDIR /

COPY --from=builder /opt/app/webtail .

EXPOSE 8080
ENTRYPOINT ["/webtail"]
