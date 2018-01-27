all: godep build test

godep:
	go get github.com/go-yaml/yaml
	go get github.com/qiniu/log
	go get github.com/urfave/cli
	go get github.com/gorilla/mux

build:
	find . -type f -name '*.go' | xargs gofmt -s -w
	go build -o bin/huker cmd/huker.go

test:
	go get github.com/go-playground/overalls
	go get github.com/mattn/goveralls
	overalls -project=github.com/openinx/huker -covermode=count -ignore='.git,_vendor'

travis-test: build test
	goveralls -coverprofile=overalls.coverprofile -service=travis-ci

clean:
	rm -rf bin/*
	rm -rf log/*
	rm -rf *.coverprofile
