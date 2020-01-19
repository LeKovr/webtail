
# cloud.docker.com do not use ARG, we do now use hooks
# ARG golang_version
# FROM golang:$golang_version

FROM golang:1.13.5-alpine3.11 as builder

WORKDIR /opt/app
RUN apk --update add curl git make

# Cached layer
COPY ./go.mod ./go.sum ./
RUN go mod download

# Sources dependent layer
COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=`git describe --tags --always`" -a ./cmd/webtail/


FROM scratch

VOLUME /data

WORKDIR /

COPY --from=builder /opt/app/webtail .

EXPOSE 8080
ENTRYPOINT ["/webtail"]
