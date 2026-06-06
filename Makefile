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

# Configuration for `make run`.
SUDO             = sudo
RUN_LDAP_USER    = ldap
RUN_LDAP_GROUP   = ldap
RUN_SERVER_USER  = portunus
RUN_SERVER_GROUP = portunus

# For running Portunus during the Edit-Compile-Run cycle.
# This target will build as your regular user and then execute the orchestrator as root.
run: all
	@$(SUDO) $(MAKE) __run

# This target is not intended to be invoked directly. Use `make run` instead.
__run:
	@env \
	PORTUNUS_DEBUG=false \
	PORTUNUS_LDAP_SUFFIX='dc=example,dc=com' \
	PORTUNUS_SERVER_BINARY="$(CURDIR)/build/portunus-server" \
	PORTUNUS_SERVER_GROUP=$(RUN_SERVER_GROUP) \
	PORTUNUS_SERVER_USER=$(RUN_SERVER_USER) \
	PORTUNUS_SERVER_HTTP_LISTEN=127.0.0.1:8080 \
	PORTUNUS_SERVER_HTTP_SECURE=false \
	PORTUNUS_SERVER_TRACER_LISTEN=127.0.0.1:3890 \
	PORTUNUS_SERVER_STATE_DIR=/var/lib/portunus \
	PORTUNUS_SLAPD_BINARY="$(shell which slapd)" \
	PORTUNUS_SLAPD_GROUP=$(RUN_LDAP_GROUP) \
	PORTUNUS_SLAPD_USER=$(RUN_LDAP_USER) \
	PORTUNUS_SLAPD_SCHEMA_DIR=/etc/openldap/schema \
	PORTUNUS_SLAPD_STATE_DIR=/run/portunus-slapd \
	./build/portunus-orchestrator

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
