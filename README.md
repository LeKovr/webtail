# webtail

[![GoCard][1]][2]
[![GitHub license][3]][4]

[1]: https://goreportcard.com/badge/LeKovr/webtail
[2]: https://goreportcard.com/report/github.com/LeKovr/webtail
[3]: https://img.shields.io/badge/license-MIT-blue.svg
[4]: LICENSE

[webtail](https://github.com/LeKovr/webtail) - Tail [log]files via websocket

This service loads list of logfiles from directory tree & continuously shows result of chosen file tail via websocket.

Project status: MVP

![Ping stream sample](webtail-ping.png)

## Install

```bash
go get github.com/LeKovr/webtail
```

### Download binary

See [Latest release](https://github.com/LeKovr/webtail/releases/latest)

### Docker

```
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
    ...
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
