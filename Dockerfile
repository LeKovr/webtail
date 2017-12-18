
# cloud.docker.com do not use ARG, we do now use hooks
# ARG golang_version
# FROM golang:$golang_version

FROM golang:1.9.2-alpine3.6

MAINTAINER Alexey Kovrizhkin <lekovr+docker@gmail.com>

# alpine does not have these apps
RUN apk add --no-cache make bash git curl

WORKDIR /go/src/github.com/LeKovr/webtail
COPY cmd cmd
COPY html html
COPY tailer tailer
COPY worker worker
COPY Makefile .
COPY glide.* ./

RUN go get -u github.com/golang/lint/golint
RUN make vendor
RUN make build-standalone

FROM scratch

VOLUME /data

WORKDIR /
COPY --from=0 /go/src/github.com/LeKovr/webtail/webtail .

EXPOSE 8080
ENTRYPOINT ["/webtail"]
