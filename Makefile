# exam project makefile

SHELL          = /bin/bash

# -----------------------------------------------------------------------------
# Build config

GO            ?= go
# not supported in BusyBox v1.26.2
SOURCES        = manager/*.go api/*.go
LIBS           = $(shell $(GO) list ./... | grep -vE '/(vendor|cmd)/')

OS            ?= linux
ARCH          ?= amd64
STAMP         ?= $$(date +%Y-%m-%d_%H:%M.%S)
ALLARCH       ?= "linux/amd64 linux/386 darwin/386"
DIRDIST       ?= dist

# -----------------------------------------------------------------------------
# Docker image config

# application name, docker-compose prefix
PRG           ?= $(shell basename $$PWD)

# Hardcoded in docker-compose.yml service name
DC_SERVICE    ?= app

# Generated docker image
DC_IMAGE      ?= webtail

# docker/compose version
DC_VER        ?= 1.14.0

# golang image version
GO_VER        ?= 1.9.2-alpine3.6

# docker app for change inside containers
DOCKER_BIN    ?= docker

# docker app log files directory
LOG_DIR       ?= /var/log

# -----------------------------------------------------------------------------
# App config

# Docker container port
SERVER_PORT   ?= 8080

# -----------------------------------------------------------------------------

.PHONY: all doc gen build-standalone coverage cov-html build test lint fmt vet vendor up down build-docker clean-docker

##
## Available targets are:
##

# default: show target list
all: help

## build and run in foreground
run: build
	./$(PRG) --log_level debug --root log/ --host localhost

## Generate protobuf & kvstore mock
gen:
	$(GO) generate ./cmd/webtail/...

doc:
	@echo "Open http://localhost:6060/pkg/LeKovr/webtail"
	@godoc -http=:6060

## Build cmds for scratch docker
build-standalone: lint vet coverage
	[ -d .git ] && GH=`git rev-parse HEAD` || GH=nogit ; \
	  GOOS=linux CGO_ENABLED=0 $(GO) build -a -v -o $(PRG) -ldflags \
	  "-X main.Build=$(STAMP) -X main.Commit=$$GH" ./cmd/$(PRG)

## Build cmds
build: gen $(PRG)

## Build webtail command
$(PRG): cmd/webtail/*.go $(SOURCES)
	[ -d .git ] && GH=`git rev-parse HEAD` || GH=nogit ; \
	  GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -v -o $@ -ldflags \
	  "-X main.Build=$(STAMP) -X main.Commit=$$GH" ./cmd/$@

## Show coverage
coverage:
	@for f in $(LIBS) ; do pushd $$GOPATH/src/$$f > /dev/null ; $(GO) test -coverprofile=coverage.out ; popd > /dev/null ; done

## Show package coverage in html (make cov-html PKG=counter)
cov-html:
	pushd $(PKG) ; $(GO) tool cover -html=coverage.out ; popd

## Run tests
test:
	$(GO) test $(LIBS)

## Run lint
lint:
	golint api/...
	golint manager/...
	golint cmd/...

## Format go sources
fmt:
	$(GO) fmt ./api/... && $(GO) fmt ./manager/... && $(GO) fmt ./cmd/...

## Run vet
vet:
	$(GO) vet ./api/... && $(GO) vet ./manager/... && $(GO) vet ./cmd/...

## Install vendor deps
vendor:
	@echo "*** $@:glide ***"
	which glide > /dev/null || curl https://glide.sh/get | sh
	@echo "*** $@ ***"
	glide install

# ------------------------------------------------------------------------------

## build app for all platforms
buildall: lint vet
	@echo "*** $@ ***"
	@[ -d .git ] && GH=`git rev-parse HEAD` || GH=nogit ; \
	  for a in "$(ALLARCH)" ; do \
	    echo "** $${a%/*} $${a#*/}" ; \
	    P=$(PRG)_$${a%/*}_$${a#*/} ; \
	    GOOS=$${a%/*} GOARCH=$${a#*/} $(GO) build -o $$P -ldflags \
	      "-X main.Build=$(STAMP) -X main.Commit=$$GH" ./cmd/$(PRG) ; \
	  done

## create disro files
dist: clean buildall
	@echo "*** $@ ***"
	@[ -d $(DIRDIST) ] || mkdir $(DIRDIST)
	@sha256sum $(PRG)_* > $(DIRDIST)/SHA256SUMS ; \
	  for a in "$(ALLARCH)" ; do \
	    echo "** $${a%/*} $${a#*/}" ; \
	    P=$(PRG)_$${a%/*}_$${a#*/} ; \
	    zip "$(DIRDIST)/$$P.zip" "$$P" README.md ; \
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

# ------------------------------------------------------------------------------
# Docker part
# ------------------------------------------------------------------------------

## Start service in container
up:
up: CMD=up -d $(DC_SERVICE)
up: dc

## Stop service
down:
down: CMD=rm -f -s $(DC_SERVICE)
down: dc

## Build docker image
build-docker:
	@$(MAKE) -s dc CMD="build --no-cache --force-rm $(DC_SERVICE)"

# Remove docker image & temp files
clean-docker:
	[[ "$$($(DOCKER_BIN) images -q $(DC_IMAGE) 2> /dev/null)" == "" ]] || $(DOCKER_BIN) rmi $(DC_IMAGE)

# ------------------------------------------------------------------------------

# $$PWD используется для того, чтобы текущий каталог был доступен в контейнере по тому же пути
# и относительные тома новых контейнеров могли его использовать
## run docker-compose
dc: docker-compose.yml
	@$(DOCKER_BIN) run --rm  -i \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $$PWD:$$PWD \
  -w $$PWD \
  --env=golang_version=$(GO_VER) \
  --env=SERVER_PORT=$(SERVER_PORT) \
  --env=LOG_DIR=$(LOG_DIR) \
  --env=DC_IMAGE=$(DC_IMAGE) \
  docker/compose:$(DC_VER) \
  -p $(PRG) \
  $(CMD)

## Show available make targets
help:
	@grep -A 1 "^##" Makefile | less
