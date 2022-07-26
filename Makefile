CMDS = orchestrator server

PREFIX        = /usr
GO_BUILDFLAGS =
GO_LDFLAGS    =

all: build/orchestrator build/server

build/%: internal/static/bindata.go FORCE
	go build -o $@ $(GO_BUILDFLAGS) -ldflags '-s -w $(GO_LDFLAGS)' 'github.com/majewsky/portunus/cmd/$*'

internal/static/bindata.go: $(shell find static -type f)
	go-bindata -ignore '\.scss$$' -prefix static/ -o $@ static/...
static/css/portunus.css: static/css/*.scss
	sassc -t compressed -I vendor/github.com/majewsky/xyrillian.css -I static/css static/css/portunus.scss static/css/portunus.css

install: FORCE all
	install -D -m 0755 "build/orchestrator" "$(DESTDIR)$(PREFIX)/bin/portunus-orchestrator"
	install -D -m 0755 "build/server"       "$(DESTDIR)$(PREFIX)/bin/portunus-server"
	install -D -m 0644 README.md            "$(DESTDIR)$(PREFIX)/share/doc/portunus/README.md"

vendor: FORCE
	go mod tidy
	go mod verify
	go mod vendor

.PHONY: FORCE
