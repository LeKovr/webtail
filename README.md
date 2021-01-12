# webtail

[![Go Reference][ref1]][ref2]
 [![codecov][cc1]][cc2]
 [![GoCard][gc1]][gc2]
 [![Build Status][bs1]][bs2]
 [![GitHub Release][gr1]][gr2]
 [![Docker Image][di1]][di2]
 [![GitHub license][gl1]][gl2]

[ref1]: https://pkg.go.dev/badge/github.com/LeKovr/webtail.svg
[ref2]: https://pkg.go.dev/github.com/LeKovr/webtail
[cc1]: https://codecov.io/gh/LeKovr/webtail/branch/master/graph/badge.svg
[cc2]: https://codecov.io/gh/LeKovr/webtail
[gc1]: https://goreportcard.com/badge/github.com/LeKovr/webtail
[gc2]: https://goreportcard.com/report/github.com/LeKovr/webtail
[bs1]: https://cloud.drone.io/api/badges/LeKovr/webtail/status.svg
[bs2]: https://cloud.drone.io/LeKovr/webtail
[gr1]: https://img.shields.io/github/release/LeKovr/webtail.svg
[gr2]: https://github.com/LeKovr/webtail/releases
[di1]: https://images.microbadger.com/badges/image/lekovr/webtail.svg
[di2]: https://microbadger.com/images/lekovr/webtail
[gl1]: https://img.shields.io/github/license/LeKovr/webtail.svg
[gl2]: https://github.com/LeKovr/webtail/blob/master/LICENSE

[webtail](https://github.com/LeKovr/webtail) - Tail [log]files via websocket

This service loads list of logfiles from directory tree & continuously shows result of chosen file tail via websocket.

Project status: MVP

![Ping stream sample](webtail-ping.png)

## Install

```sh
go get github.com/LeKovr/webtail
```

### Download binary

See [Latest release](https://github.com/LeKovr/webtail/releases/latest)

### Docker

```sh
docker pull lekovr/webtail
```

See [docker-compose.yml](docker-compose.yml) for usage example.

## Embed package in your service

```go
package main
import (
    "github.com/LeKovr/webtail"
)

func main() {
    var wt *webtail.Service
    wt, err = webtail.New(log, cfg.WebTail)
    if err != nil {
        return
    }
    go wt.Run()
    // ...
    http.HandleFunc("/tail", func(w http.ResponseWriter, r *http.Request) {
        wt.Handle(w, r)
    })
}
```

## Note about gorilla/websocket

Starting from v0.30 this code is based on [gorilla/websocket chat example](https://github.com/gorilla/websocket/tree/master/examples/chat). See cmd/webtail/{client,hub}.go

## TODO

* [x] js: add mask for row coloring
* [ ] add tests & more docs
* [ ] add text field for server-side log filtering

## License

The MIT License (MIT), see [LICENSE](LICENSE).

Copyright (c) 2016-2021 Aleksey Kovrizhkin <lekovr+webtail@gmail.com>
