.PHONY: build test vet lint clean install release-dry

VERSION ?= dev

build:
	go build -ldflags "-s -w -X ctm/cmd.Version=$(VERSION)" -o ctm .

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint: vet
	@echo "lint ok"

clean:
	rm -f ctm

install: build
	./ctm install

release-dry:
	goreleaser release --snapshot --clean

all: vet test build
