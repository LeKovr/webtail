FROM golang:latest

MAINTAINER Alexey Kovrizhkin <lekovr+docker@gmail.com>

WORKDIR /go/src/app
COPY . .

RUN go-wrapper download
RUN CGO_ENABLED=0 GOOS=linux go build -a -o webtail

FROM scratch

WORKDIR /
COPY --from=0 /go/src/app/webtail .

EXPOSE 8080
ENTRYPOINT ["/webtail"]
