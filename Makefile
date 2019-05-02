PLATFORM	= $(shell uname | tr 'A-Z' 'a-z')
ARCH		= amd64
DEP		= ./.dep
DEP_VERSION	= 0.3.2
OUTPUT		= bin/bub

.PHONY: all dev deps test clean release fmt

all: clean deps test darwin linux

dev:
	GOOS=darwin GOARCH=$(ARCH) go build -i -o "$(OUTPUT)-$(PLATFORM)-$(ARCH)"

darwin:
	GOOS=darwin GOARCH=$(ARCH) go build -o "$(OUTPUT)-darwin-$(ARCH)"

linux:
	GOOS=linux GOARCH=$(ARCH) go build -o "$(OUTPUT)-linux-$(ARCH)"

$(DEP):
	curl --fail --silent --location \
		"https://github.com/golang/dep/releases/download/v$(DEP_VERSION)/dep-$(PLATFORM)-amd64" \
		--output "$(DEP)"
	chmod +x "$(DEP)"

deps: $(DEP)
	$(DEP) ensure --vendor-only

test:
	go test ./...

clean:
	rm -rf bin

release: all
	$(eval version := $(shell bin/bub-$(PLATFORM)-$(ARCH) --version | sed 's/ version /-/g'))
	git tag $(version)
	find bin -type f -exec gzip --keep {} \;
	find bin -type f -name *.gz -exec shasum -a 256 {} \;

install: deps dev
	rm -f /usr/local/bin/bub
	ln -s -f $(shell pwd)/bin/bub-$(PLATFORM)-$(ARCH) /usr/local/bin/bub

fmt:
	go fmt ./...
