PKG  = github.com/majewsky/portunus
CMDS = orchestrator server
PREFIX=/usr

all: $(addprefix build/,$(CMDS))

# NOTE: This repo uses Go modules, and uses a synthetic GOPATH at
# $(CURDIR)/.gopath that is only used for the build cache. $GOPATH/src/ is
# empty.
GO            = GOPATH=$(CURDIR)/.gopath GOBIN=$(CURDIR)/build go
GO_BUILDFLAGS =
GO_LDFLAGS    = -s -w

build/%: FORCE
	$(GO) install $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' '$(PKG)/cmd/$*'

install: FORCE all
	for CMD in $(CMDS); do install -D -m 0755 "build/$${CMD}" "$(DESTDIR)$(PREFIX)/bin/portunus-$${CMD}"; done
	install -D -m 0644 README.md "$(DESTDIR)$(PREFIX)/share/doc/portunus/README.md"

vendor: FORCE
	$(GO) mod vendor

.PHONY: FORCE
