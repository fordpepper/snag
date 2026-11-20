PREFIX ?= /usr/local

build:
	go build -ldflags "-s -w" -o snag .

install: build
	install -Dm755 snag $(DESTDIR)$(PREFIX)/bin/snag

clean:
	rm -f snag

.PHONY: build install clean
