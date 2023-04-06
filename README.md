# webtail

> Tail [log]files via websocket

<p align="center">
  <span>English</span> |
  <a href="README.ru.md#readme">Pусский</a>
</p>

---

[![Go Reference][ref1]][ref2]
 [![GitHub Release][gr1]][gr2]
 [![Build Status][bs1]][bs2]
 [![GitHub license][gl1]][gl2]

[![Go Coverage][cc1]][cc2]
 [![Test Coverage][cct1]][cct2]
 [![Maintainability][ccm1]][ccm2]
 [![GoCard][gc1]][gc2]

[cct1]: https://api.codeclimate.com/v1/badges/909eca87d9ee5b216a6b/test_coverage
[cct2]: https://codeclimate.com/github/LeKovr/webtail/test_coverage
[ccm1]: https://api.codeclimate.com/v1/badges/909eca87d9ee5b216a6b/maintainability
[ccm2]: https://codeclimate.com/github/LeKovr/webtail/maintainability
[ref1]: https://pkg.go.dev/badge/github.com/LeKovr/webtail.svg
[ref2]: https://pkg.go.dev/github.com/LeKovr/webtail
[cc1]: https://github.com/LeKovr/webtail/wiki/coverage.svg
[cc2]: https://raw.githack.com/wiki/LeKovr/webtail/coverage.html
[gc1]: https://goreportcard.com/badge/github.com/LeKovr/webtail
[gc2]: https://goreportcard.com/report/github.com/LeKovr/webtail
[bs1]: https://github.com/LeKovr/webtail/actions/workflows/docker-publish.yml/badge.svg
[bs2]: http://github.com/LeKovr/webtail/actions/workflows/docker-publish.yml
[gr1]: https://img.shields.io/github/release/LeKovr/webtail.svg
[gr2]: https://github.com/LeKovr/webtail/releases
[gl1]: https://img.shields.io/github/license/LeKovr/webtail.svg
[gl2]: https://github.com/LeKovr/webtail/blob/master/LICENSE

[webtail](https://github.com/LeKovr/webtail) is a web-service and golang package used for continious updated files publication via websocker to browser.

![Ping stream sample](screenshot.png)

## Install

```sh
go get -v github.com/LeKovr/webtail/...
```

### Download binary

See [Latest release](https://github.com/LeKovr/webtail/releases/latest)

### Docker

Starting from 0.43.2 docker images are published at [GitHub Packages](https://ghcr.io), so use

```sh
docker pull ghcr.io/lekovr/webtail:latest
```

See [docker-compose.yml](docker-compose.yml) for usage example.

v0.43.1 is the [last version available at dockerhub](https://hub.docker.com/repository/docker/lekovr/webtail/tags).

## Use package in your service

```go
package main
import (
    "github.com/LeKovr/webtail"
)

func main() {
    wt, err := webtail.New(log, cfg)
    if err != nil {
        return
    }
    go wt.Run()
    defer wt.Close()
    // ...
    http.Handle("/tail", wt)
}
```

See also: [app.go](https://github.com/LeKovr/webtail/blob/master/cmd/webtail/app.go)

## Note about gorilla/websocket

Starting from v0.30 this code is based on [gorilla/websocket chat example](https://github.com/gorilla/websocket/tree/master/examples/chat). See {client,hub}.go

## License

The MIT License (MIT), see [LICENSE](LICENSE).

Copyright (c) 2016-2021 Aleksey Kovrizhkin <lekovr+webtail@gmail.com>
