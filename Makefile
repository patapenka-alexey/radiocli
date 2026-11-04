BINARY  := radiocli
GO      ?= go
GOFMT   ?= gofmt
PREFIX  ?= /usr/local
BINDIR  ?= $(PREFIX)/bin
DESTDIR ?=

.PHONY: all build install uninstall clean run fmt fmt-check vet test test-race check

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

fmt-check:
	@files="$$($(GOFMT) -l .)"; if [ -n "$$files" ]; then \
		echo "gofmt required for:" >&2; \
		echo "$$files" >&2; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

check: fmt-check vet test-race
