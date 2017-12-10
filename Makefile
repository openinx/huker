all: godep build

godep:
	@go get github.com/go-yaml/yaml

build:
	@go build -o bin/huker-pkg cmd/huker-pkg.go
	@go build -o bin/huker-cli cmd/huker-cli.go

test:
	@go test ./haloop/...

clean:
	@rm -rf bin
	@rm -rf log/*
	@rm -rf Godeps
