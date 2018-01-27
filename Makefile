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
	@go get github.com/go-playground/overalls
	@go get github.com/mattn/goveralls
	@overalls -project=github.com/openinx/huker -covermode=count -ignore='.git,_vendor'
	@goveralls -coverprofile=overalls.coverprofile -service=travis-ci

clean:
	@rm -rf bin/*
	@rm -rf log/*
	@rm -rf *.coverprofile
