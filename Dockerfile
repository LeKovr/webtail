
ARG golang_version

FROM golang:$golang_version

MAINTAINER Alexey Kovrizhkin <lekovr+docker@gmail.com>

WORKDIR /go/src/github.com/LeKovr/webtail
COPY cmd cmd
COPY html html
COPY manager manager
COPY api api
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
