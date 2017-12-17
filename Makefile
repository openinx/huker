all: godep build

godep:
	@go get github.com/go-yaml/yaml

build:
	@find . -name '*.go' | xargs gofmt -w
	@go build -o bin/huker cmd/huker.go

test:
	@go test ./...

clean:
	@rm -rf bin/*
	@rm -rf log/*
