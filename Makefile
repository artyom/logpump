export GOPATH := $(PWD)/.build
PPATH := github.com/artyom/logpump
TMPSRC := $(GOPATH)/src/$(PPATH)

all: logpump

logpump: logpump.go
	mkdir -p $(GOPATH)/src $(GOPATH)/bin $(GOPATH)/pkg
	mkdir -p $(TMPSRC)
	cp -av $< $(TMPSRC)
	go get -v $(PPATH)
	go build -v $(PPATH)

clean:
	find . -mindepth 1 -maxdepth 1 -name logpump -type f -executable -delete
	rm -rf $(GOPATH)

install: logpump
	install -d -m0755 -oroot -groot $(DESTDIR)/usr/bin
	install -v -m0755 -oroot -groot $< $(DESTDIR)/usr/bin

.PHONY: all clean install
