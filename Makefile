CMDS = portunus-orchestrator portunus-server

PREFIX        = /usr
GO_BUILDFLAGS =
GO_LDFLAGS    =

all: $(addprefix build/,$(CMDS))

build/%: static/css/portunus.css FORCE
	go build -o $@ $(GO_BUILDFLAGS) -ldflags '-s -w $(GO_LDFLAGS)' 'github.com/majewsky/portunus/cmd/$*'

static/css/portunus.css: static/css/*.scss
	sassc -t compressed -I vendor/github.com/majewsky/xyrillian.css -I static/css static/css/portunus.scss static/css/portunus.css

install: FORCE all
	install -D -m 0755 "build/portunus-orchestrator" "$(DESTDIR)$(PREFIX)/bin/portunus-orchestrator"
	install -D -m 0755 "build/portunus-server"       "$(DESTDIR)$(PREFIX)/bin/portunus-server"
	install -D -m 0644 README.md                     "$(DESTDIR)$(PREFIX)/share/doc/portunus/README.md"

check: build/cover.html

build/cover.out: FORCE | build
	@printf "\e[1;36m>> go test\e[0m\n"
	@go test $(GO_BUILDFLAGS) -ldflags '-s -w $(GO_LDFLAGS)' -shuffle=on -p 1 -coverprofile=$@ -covermode=count 'github.com/majewsky/portunus/...'

build/cover.html: build/cover.out
	@printf "\e[1;36m>> go tool cover > build/cover.html\e[0m\n"
	@go tool cover -html $< -o $@

build:
	@mkdir $@

vendor: FORCE
	go mod tidy
	go mod verify
	go mod vendor
	@# need to move these files into static/ to enable embedding into the binary
	rm -f -- static/fonts/*.otf && cp -t static/fonts/ vendor/github.com/majewsky/xyrillian.css/Raleway-*.otf

.PHONY: FORCE
