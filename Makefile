## webtail Makefile:
## Tail [log]files via web
#:

SHELL          = /bin/sh
PRG           ?= $(shell basename $$PWD)

# -----------------------------------------------------------------------------
# Build config

GO            ?= go
# not supported in BusyBox v1.26.2
SOURCES        = $(shell find . -maxdepth 3 -mindepth 1 -name '*.go'  -printf '%p\n')
APP_VERSION   ?= $(shell git describe --tags --always)
GOLANG_VERSION = 1.15.5-alpine3.12

OS            ?= linux
ARCH          ?= amd64
ALLARCH       ?= "linux/amd64 linux/386 darwin/amd64 linux/arm linux/arm64"
DIRDIST       ?= dist

# Path to golang package docs
GODOC_REPO    ?= github.com/!le!kovr/$(PRG)
# App docker image
DOCKER_IMAGE  ?= ghcr.io/lekovr/$(PRG)

# -----------------------------------------------------------------------------
# Docker image config

# Hardcoded in docker-compose.yml service name
DC_SERVICE    ?= app

# docker-compose image version
DC_VER        ?= latest

# docker app for change inside containers
DOCKER_BIN    ?= docker

# docker app log files directory
LOG_DIR       ?= ./log

# -----------------------------------------------------------------------------
# App config

# Docker container port
SERVER_PORT   ?= 8080

# -----------------------------------------------------------------------------

.PHONY: all doc gen build-standalone coverage cov-html build test lint fmt vet vendor up down docker-docker docker-clean

# default: show target list
all: help

# ------------------------------------------------------------------------------
## Compile operations
#:

## Run lint
lint:
	@golint ./...
	@golangci-lint run ./...

## Run vet
vet:
	$(GO) vet ./...

## Run tests
test: coverage.out

coverage.out: $(SOURCES)
	$(GO) test -tags test -race -covermode=atomic -coverprofile=$@ ./...

## Show package coverage in html (make cov-html PKG=counter)
cov-html: coverage.out
	$(GO) tool cover -html=coverage.out

## Build app
build: $(PRG)

## Build webtail command
$(PRG): $(SOURCES)
	GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -v -o $@ -ldflags \
	  "-X main.version=$(APP_VERSION)" ./cmd/$@

## Build like docker image from scratch
build-standalone: lint vet test
	GOOS=linux CGO_ENABLED=0 $(GO) build -a -v -o $(PRG) -ldflags \
	  "-X main.version=$(APP_VERSION)" ./cmd/$(PRG)

## build and run in foreground
run: build
	./$(PRG) --debug --root log/ --html html --trace

run-abs: build
	./$(PRG) --debug --root $$PWD/log/ --html html --trace

doc:
	@echo "Open http://localhost:6060/pkg/LeKovr/webtail"
	@godoc -http=:6060

# ------------------------------------------------------------------------------
## Prepare distros
#:

## build app for all platforms
buildall: lint vet
	@echo "*** $@ ***" ; \
	  for a in "$(ALLARCH)" ; do \
	    echo "** $${a%/*} $${a#*/}" ; \
	    P=$(PRG)-$${a%/*}_$${a#*/} ; \
	    GOOS=$${a%/*} GOARCH=$${a#*/} $(GO) build -o $$P -ldflags \
	      "-X main.version=$(APP_VERSION)" ./cmd/$(PRG) ; \
	  done

## create disro files
dist: clean buildall
	@echo "*** $@ ***"
	@[ -d $(DIRDIST) ] || mkdir $(DIRDIST)
	@sha256sum $(PRG)-* > $(DIRDIST)/SHA256SUMS ; \
	  for a in "$(ALLARCH)" ; do \
	    echo "** $${a%/*} $${a#*/}" ; \
	    P=$(PRG)-$${a%/*}_$${a#*/} ; \
	    zip "$(DIRDIST)/$$P.zip" "$$P" README.md README.ru.md screenshot.png; \
        rm "$$P" ; \
	  done


## clean generated files
clean:
	@echo "*** $@ ***" ; \
	  for a in "$(ALLARCH)" ; do \
	    P=$(PRG)_$${a%/*}_$${a#*/} ; \
	    [ -f $$P ] && rm $$P || true ; \
	  done
	@[ -d $(DIRDIST) ] && rm -rf $(DIRDIST) || true
	@[ -f $(PRG) ] && rm -f $(PRG) || true
	@[ ! -f coverage.out ] || rm coverage.out

# ------------------------------------------------------------------------------
## Docker operations
#:

## Start service in container
up:
up: CMD="up -d $(DC_SERVICE)"
up: dc

## Stop service
down:
down: CMD="rm -f -s $(DC_SERVICE)"
down: dc

## Build docker image
docker-build: CMD="build --no-cache --force-rm $(DC_SERVICE)"
docker-build: dc

## Remove docker image & temp files
docker-clean:
	[ "$$($(DOCKER_BIN) images -q $(DC_IMAGE) 2> /dev/null)" = "" ] || $(DOCKER_BIN) rmi $(DC_IMAGE)

# ------------------------------------------------------------------------------

# $$PWD usage allows host directory mounts in child containers
# Thish works if path is the same for host, docker, docker-compose and child container
## run $(CMD) via docker-compose
dc: docker-compose.yml
	@$(DOCKER_BIN) run --rm  -i \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $$PWD:$$PWD -w $$PWD \
  --env=SERVER_PORT=$(SERVER_PORT) \
  --env=LOG_DIR=$(LOG_DIR) \
  --env=DOCKER_IMAGE=$(DOCKER_IMAGE) \
  --env=GOLANG_VERSION=$(GOLANG_VERSION) \
  docker/compose:$(DC_VER) \
  -p $(PRG) \
  "$(CMD)"

# ------------------------------------------------------------------------------
## Other
#:

## update docs at pkg.go.dev
godoc:
	vf=$(APP_VERSION) ; v=$${vf%%-*} ; echo "Update for $$v..." ; \
	curl 'https://proxy.golang.org/$(GODOC_REPO)/@v/'$$v'.info'

## update latest docker image tag at ghcr.io
ghcr:
	vf=$(APP_VERSION) ; vs=$${vf%%-*} ; v=$${vs#v} ; echo "Update for $$v..." ; \
	docker pull $(DOCKER_IMAGE):$$v && \
	docker tag $(DOCKER_IMAGE):$$v $(DOCKER_IMAGE):latest && \
	docker push $(DOCKER_IMAGE):latest

# This code handles group header and target comment with one or two lines only
## list Makefile targets
## (this is default target)
help:
	@grep -A 1 -h "^## " $(MAKEFILE_LIST) \
  | sed -E 's/^--$$// ; /./{H;$$!d} ; x ; s/^\n## ([^\n]+)\n(## (.+)\n)*(.+):(.*)$$/"    " "\4" "\1" "\3"/' \
  | sed -E 's/^"    " "#" "(.+)" "(.*)"$$/"" "" "" ""\n"\1 \2" "" "" ""/' \
  | xargs printf "%s\033[36m%-15s\033[0m %s %s\n"
