all: godep build

godep:
	@go get github.com/go-yaml/yaml

build:
	@go build -o bin/huker-pkg cmd/huker-pkg.go
	@go build -o bin/huker-agent cmd/huker-agent.go

test:
	@go test ./haloop/...

agent:
	@go build -o bin/huker-agent cmd/huker-agent.go
	@./bin/huker-agent


clean:
	@rm -rf bin/*
	@rm -rf log/*
