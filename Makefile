.PHONY: build test vet install clean

build:
	go build -o bin/task ./cmd/task

test:
	go test ./...

vet:
	go vet ./...

install: build
	go install ./cmd/task
	cp ./bin/task /Users/shaun/scripts/task

clean:
	rm -rf bin
