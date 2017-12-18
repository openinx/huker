all: godep build

godep:
	@go get github.com/go-yaml/yaml
	@go get github.com/qiniu/log
	@go get github.com/urfave/cli
	@go get github.com/gorilla/mux

build:
	@find . -name '*.go' | xargs gofmt -w
	@go build -o bin/huker cmd/huker.go

test:
	@go test ./...

clean:
	@rm -rf bin/*
	@rm -rf log/*
