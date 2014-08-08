export GOPATH := $(PWD)/.build
PPATH := github.com/artyom/logpump
TMPSRC := $(GOPATH)/src/$(PPATH)

all: logpump

logpump: logpump.go
	mkdir -p $(GOPATH)/src $(GOPATH)/bin $(GOPATH)/pkg
	find _vendor/src -maxdepth 1 -mindepth 1 -type d -exec cp -a '{}' $(GOPATH)/src \;
	mkdir -p $(TMPSRC)
	cp -av $< $(TMPSRC)
	go build -v $(PPATH)

clean:
	find . -mindepth 1 -maxdepth 1 -name logpump -type f -executable -delete
	rm -rf $(GOPATH)

install: logpump
	install -d -m0755 -oroot -groot $(DESTDIR)/usr/bin
	install -v -m0755 -oroot -groot $< $(DESTDIR)/usr/bin

.PHONY: all clean install
