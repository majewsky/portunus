PKG = github.com/majewsky/portunus
PREFIX=/usr

all: build/portunus

# NOTE: This repo uses Go modules, and uses a synthetic GOPATH at
# $(CURDIR)/.gopath that is only used for the build cache. $GOPATH/src/ is
# empty.
GO            = GOPATH=$(CURDIR)/.gopath GOBIN=$(CURDIR)/build go
GO_BUILDFLAGS =
GO_LDFLAGS    = -s -w

build/portunus: FORCE
	$(GO) install $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' '$(PKG)'

install: FORCE all
	install -D -m 0755 build/portunus "$(DESTDIR)$(PREFIX)/bin/portunus"
	install -D -m 0644 README.md      "$(DESTDIR)$(PREFIX)/share/doc/portunus/README.md"

vendor: FORCE
	$(GO) mod vendor

.PHONY: FORCE
