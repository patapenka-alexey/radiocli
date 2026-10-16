BINARY  := radiocli
GO      ?= go
PREFIX  ?= /usr/local
BINDIR  ?= $(PREFIX)/bin
DESTDIR ?=

.PHONY: all build install uninstall clean run fmt vet test

all: build

build:
	$(GO) build -o $(BINARY) .

# install does not build, so it can run under sudo without needing Go on
# root's PATH. Run `make` first, then `sudo make install`.
install:
	@if [ ! -x $(BINARY) ]; then \
		echo "$(BINARY) not found; run 'make build' first (never build as root)" >&2; \
		exit 1; \
	fi
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

clean:
	rm -f $(BINARY)

run:
	$(GO) run .

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...
