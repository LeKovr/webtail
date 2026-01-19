## webtail Makefile:
## Tail [log]files via web
#:

SHELL          = /bin/sh
PRG           ?= $(shell basename $$PWD)
PRG_DEST      ?= $(PRG)
# -----------------------------------------------------------------------------
# Build config

GO            ?= go
GOLANG_VERSION = v1.23.6-alpine3.21.3

SOURCES        = $(shell find . -maxdepth 3 -mindepth 1 -path ./var -prune -o -name '*.go')
APP_VERSION   ?= $(shell git describe --tags --always)
# Last project tag (used in `make changelog`)
RELEASE       ?= $(shell git describe --tags --abbrev=0 --always)
# Repository address (compiled into main.repo)
REPO          ?= $(shell git config --get remote.origin.url)

TARGETOS      ?= linux
TARGETARCH    ?= amd64
LDFLAGS       := -s -w -extldflags '-static'

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

# docker-compose image and version
DC_IMAGE      ?= docker/compose
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

## Build app
build: $(PRG)

$(PRG): $(SOURCES)
	GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -v -o $@ -ldflags \
	  "-X main.version=$(APP_VERSION) -X main.repo=$(REPO)" ./cmd/$@

## Build like docker image from scratch
build-standalone: test
	CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
	  $(GO) build -a -o $(PRG_DEST) \
	  -ldflags "${LDFLAGS} -X main.version=$(APP_VERSION) -X main.repo=$(REPO)" \
	  ./cmd/$(PRG)

## build and run in foreground
run: $(PRG)
	./$(PRG) --log.debug --root log/ --html html --trace

run-abs: $(PRG)
	./$(PRG) --log.debug --root $$PWD/log/ --html html --trace

## Format go sources
fmt:
	$(GO) fmt ./...

## Run lint
lint:
	@which golint > /dev/null || go install golang.org/x/lint/golint@latest
	@golint ./...

## Run golangci-lint
ci-lint:
	@golangci-lint run ./...

## Run vet
vet:
	@$(GO) vet ./...

## Run tests
test: lint vet coverage.out

# internal target
coverage.out: $(SOURCES)
	$(GO) test -tags test -race -covermode=atomic -coverprofile=$@ ./...

## Show code coverage in html (make cov-html PKG=counter)
cov-html: coverage.out
	$(GO) tool cover -html=coverage.out

## Show code coverage per func
cov-func: coverage.out
	$(GO) tool cover -func coverage.out

## Show total code coverage
cov-total: coverage.out
	@$(GO) tool cover -func coverage.out | grep total: | awk '{print $$3}'
doc:
	@echo "Open http://localhost:6060/pkg/LeKovr/webtail"
	@godoc -http=:6060

## Changes from last tag
changelog:
	@echo Changes since $(RELEASE)
	@echo
	@git log $(RELEASE)..@ --pretty=format:"* %s"

# ------------------------------------------------------------------------------
## Prepare distros
#:

## build app for all platforms
buildall: lint vet
	@echo "*** $@ ***" ; \
	  for a in "$(ALLARCH)" ; do \
	    echo "** $${a%/*} $${a#*/}" ; \
	    P=$(PRG)-$${a%/*}_$${a#*/} ; \
	    $(MAKE) -s build-standalone TARGETOS=$${a%/*} TARGETARCH=$${a#*/} PRG_DEST=$$P ; \
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
  $(DC_IMAGE):$(DC_VER) \
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
	v=$(APP_VERSION) ; echo "Update for $$v..." ; \
	docker pull $(DOCKER_IMAGE):$$v && \
	docker tag $(DOCKER_IMAGE):$$v $(DOCKER_IMAGE):latest && \
	docker push $(DOCKER_IMAGE):latest

# linux/amd64, linux/amd64/v2, linux/amd64/v3, linux/arm64, linux/riscv64, linux/ppc64le, linux/s390x, linux/386, 
# linux/mips64le, linux/mips64, linux/arm/v7, linux/arm/v6

ALLARCH_DOCKER ?= "linux/arm/v7,linux/arm64"

OWN_HUB ?= it.elfire.ru

buildkit.toml:
	@echo [registry."$(OWN_HUB)"] > $@
	@echo ca=["/etc/docker/certs.d/$(OWN_HUB)/ca.crt"] >> $@

use-own-hub: buildkit.toml
	@docker buildx create --use --config $<

docker-multi:
	time docker buildx build --platform $(ALLARCH_DOCKER) -t $(DOCKER_IMAGE):$(APP_VERSION) --push .

# This code handles group header and target comment with one or two lines only
## list Makefile targets
## (this is default target)
help:
	@grep -A 1 -h "^## " $(MAKEFILE_LIST) \
  | sed -E 's/^--$$// ; /./{H;$$!d} ; x ; s/^\n## ([^\n]+)\n(## (.+)\n)*(.+):(.*)$$/"    " "\4" "\1" "\3"/' \
  | sed -E 's/^"    " "#" "(.+)" "(.*)"$$/"" "" "" ""\n"\1 \2" "" "" ""/' \
  | xargs printf "%s\033[36m%-15s\033[0m %s %s\n"
