ARG GOLANG_VERSION

FROM golang:$GOLANG_VERSION as builder

ARG APP_VERSION
ARG BUILD_DATE

WORKDIR /opt/app

# Cached layer
COPY ./go.mod ./go.sum ./
RUN go mod download
RUN go get github.com/go-bindata/go-bindata/...
RUN go get github.com/elazarl/go-bindata-assetfs/...

# Sources dependent layer
COPY ./ ./
RUN go generate ./cmd/webtail/...
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.built=$BUILD_DATE -X main.version=$APP_VERSION" -a ./cmd/webtail/

FROM scratch

ARG BUILD_DATE
ARG VCS_REF

LABEL maintainer="lekovr+webtail@gmail.com" \
      org.label-schema.description="Tail [log]files via web" \
      org.label-schema.schema-version="1.0" \
      org.label-schema.url="https://github.com/LeKovr/webtail" \
      org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.vcs-url="https://github.com/LeKovr/webtail.git" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.schema-version="1.0"

VOLUME /data
WORKDIR /
COPY --from=builder /opt/app/webtail .
EXPOSE 8080
ENTRYPOINT ["/webtail"]
