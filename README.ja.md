# webtail

> websocket 経由で [log] ファイルを Tail する

<p align="center">
  <a href="README.md#readme">English</a> |
  <a href="README.ru.md#readme">Pусский</a> |
  <span>日本語</span>
</p>

---

[![Go Reference][ref1]][ref2]
 [![GitHub Release][gr1]][gr2]
 [![GitHub license][gl1]][gl2]
 [![Go Coverage][cc1]][cc2]
 [![GoCard][gc1]][gc2]

[ref1]: https://pkg.go.dev/badge/github.com/LeKovr/webtail.svg
[ref2]: https://pkg.go.dev/github.com/LeKovr/webtail
[cc1]: https://github.com/LeKovr/webtail/wiki/coverage.svg
[cc2]: https://raw.githack.com/wiki/LeKovr/webtail/coverage.html
[gc1]: https://goreportcard.com/badge/github.com/LeKovr/webtail
[gc2]: https://goreportcard.com/report/github.com/LeKovr/webtail
[gr1]: https://img.shields.io/github/release/LeKovr/webtail.svg
[gr2]: https://github.com/LeKovr/webtail/releases
[gl1]: https://img.shields.io/github/license/LeKovr/webtail.svg
[gl2]: https://github.com/LeKovr/webtail/blob/master/LICENSE

[webtail](https://github.com/LeKovr/webtail) はウェブサービスと golang パッケージで、websocker 経由で継続的に更新されたファイルをブラウザに公開するために使われます。

![Ping stream sample](screenshot.png)

## インストール

```sh
go get -v github.com/LeKovr/webtail/...
```

### ダウンロード バイナリ

[最新リリース](https://github.com/LeKovr/webtail/releases/latest)を参照

### Docker

0.43.2 以降の docker イメージは、[GitHub Packages](https://ghcr.io) で公開されています

```sh
docker pull ghcr.io/lekovr/webtail:latest
```

使用例は [docker-compose.yml](docker-compose.yml) を参照。

v0.43.1 は [dockerhub で利用可能な最後のバージョン](https://hub.docker.com/repository/docker/lekovr/webtail/tags)です。

## サービスでパッケージを使用する

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

こちらも参照: [app.go](https://github.com/LeKovr/webtail/blob/master/cmd/webtail/app.go)

## gorilla/websocket に関する注意事項

v0.30 から、このコードは [gorilla/websocket チャット例](https://github.com/gorilla/websocket/tree/master/examples/chat)に基づいています。{client,hub}.go を参照

## ライセンス

MIT ライセンス (MIT)、[LICENSE](LICENSE) を参照のこと。

Copyright (c) 2016-2023 Aleksey Kovrizhkin <lekovr+webtail@gmail.com>
