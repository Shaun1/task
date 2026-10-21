.PHONY: build test vet install clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/task ./cmd/task

test:
	go test ./...

vet:
	go vet ./...

install: build
	go install -ldflags "$(LDFLAGS)" ./cmd/task
	sh ./scripts/install-completion.sh ./bin/task

clean:
	rm -rf bin
