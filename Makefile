##
## Golang application makefile
##
SHELL      = /bin/bash

# application name
PRG       ?= $(shell basename $$PWD)
SOURCES   ?= *.go
SOURCEDIR ?= "."

# Runtime data
DB_NAME   ?= dbrpc
APP_SITE  ?= localhost:8081
APP_ADDR  ?= $(APP_SITE)

# Default config
OS        ?= linux
ARCH      ?= amd64
DIRDIST   ?= dist
PRGBIN    ?= $(PRG)_$(OS)_$(ARCH)
PRGPATH   ?= $(PRGBIN)
PIDFILE   ?= $(PRGBIN).pid
LOGFILE   ?= $(PRGBIN).log
STAMP     ?= $$(date +%Y-%m-%d_%H:%M.%S)
ALLARCH   ?= "linux/amd64 linux/386 darwin/386"

# Search .git for commit id fetch
GIT_ROOT  ?= $$([ -d ./.git ] && echo "." || { [ -d ../.git ] && echo ".." ; } || { [ -d ../../.git ] && echo "../.." ; })


##
## Available targets are:
##

# default: show target list
all:
	@grep -A 1 "^##" Makefile

## build and run in foreground
run: build
	./$(PRGPATH) --log_level debug --root log/ --lines 200

## build and show program help
help: build
	./$(PRGPATH) --help

## build and show version
ver: build
	./@$(PRGPATH) --version && echo ""

## generate bindata
# See https://github.com/elazarl/go-bindata-assetfs
# for disabling lint warnings use "//golint:ignore"
bindata:
	go-bindata-assetfs html/

## build app
build: bindata lint vet $(PRGPATH)

## build app for default arch
$(PRGPATH): $(SOURCES)
	@echo "*** $@ ***"
	@[ -d $(GIT_ROOT)/.git ] && GH=`git rev-parse HEAD` || GH=nogit ; \
GOOS=$(OS) GOARCH=$(ARCH) go build -v -o $(PRGBIN) -ldflags \
"-X main.Build=$(STAMP) -X main.Commit=$$GH"

## build app for all platforms
buildall: lint vet
	@echo "*** $@ ***"
	@[ -d $(GIT_ROOT)/.git ] && GH=`git rev-parse HEAD` || GH=nogit ; \
for a in "$(ALLARCH)" ; do \
  echo "** $${a%/*} $${a#*/}" ; \
  P=$(PRG)_$${a%/*}_$${a#*/} ; \
  [ "$${a%/*}" == "windows" ] && P=$$P.exe ; \
  GOOS=$${a%/*} GOARCH=$${a#*/} go build -o $$P -ldflags \
  "-X main.Build=$(STAMP) -X main.Commit=$$GH" ; \
done

## create disro files
dist: clean buildall
	@echo "*** $@ ***"
	@[ -d $(DIRDIST) ] || mkdir $(DIRDIST) ; \
sha256sum $(PRG)* > $(DIRDIST)/SHA256SUMS ; \
for a in "$(ALLARCH)" ; do \
  echo "** $${a%/*} $${a#*/}" ; \
  P=$(PRG)_$${a%/*}_$${a#*/} ; \
  [ "$${a%/*}" == "windows" ] && P1=$$P.exe || P1=$$P ; \
  zip "$(DIRDIST)/$$P.zip" "$$P1" README.md ; \
done

test-log:
	[ -d log ] || mkdir log
	while true ; do echo -n "." ; date >> log/xxx ; sleep 2 ; done

## clean generated files
clean:
	@echo "*** $@ ***"
	@for a in "$(ALLARCH)" ; do \
  P=$(PRG)_$${a%/*}_$${a#*/} ; \
  [ "$${a%/*}" == "windows" ] && P=$$P.exe ; \
  [ -f $$P ] && rm $$P || true ; \
done ; \
[ -d $(DIRDIST) ] && rm -rf $(DIRDIST) || true

## run go lint
lint:
	@echo "*** $@ ***"
	@for d in "$(SOURCEDIR)" ; do echo $$d && golint $$d/*.go ; done

## run go vet
vet:
	@echo "*** $@ ***"
#	@for d in "$(SOURCEDIR)" ; do echo $$d && go vet $$d/*.go ; done
# does not build with go 1.7

.PHONY: all run ver buildall clean dist link vet

