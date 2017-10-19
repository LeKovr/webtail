
webtail
=======

[![GoCard][1]][2]
[![GitHub license][3]][4]

[1]: https://goreportcard.com/badge/LeKovr/webtail
[2]: https://goreportcard.com/report/github.com/LeKovr/webtail
[3]: https://img.shields.io/badge/license-MIT-blue.svg
[4]: LICENSE

[webtail](https://github.com/LeKovr/webtail) - Tail logfile via websocket

This service loads list of logfiles from directory tree & continuously shows result of chosen file tail via websocket.

Install
-------

```
go get github.com/LeKovr/webtail
```

### Download

See [Latest release](https://github.com/LeKovr/webtail/releases/latest)

Acknowledgements
----------------
* [Aman Mangal](https://github.com/mangalaman93) for his [golang tail lib](https://github.com/mangalaman93/tail)
* [stackoverflow](http://stackoverflow.com) authors & posters

### TODO

* [x] js: don't enable "follow" button on big update
* [x] go: use https://github.com/hpcloud/tail instead https://github.com/LeKovr/tail
* [ ] js: add inputs for filter plain/green/yellow/red
* [ ] go: use https://github.com/gorilla/websocket instead https://golang.org/x/net/websocket
* [ ] js: reconnect ws on close
* [ ] add text field for log filtering

License
-------

The MIT License (MIT), see [LICENSE](LICENSE).

Copyright (c) 2016 Alexey Kovrizhkin ak@elfire.ru
