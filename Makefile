HUKER_VERSION := huker-1.0.0

all: build

build:
	find . -type f -name '*.go' | xargs gofmt -s -w
	go build -o bin/huker cmd/huker.go
	go build -o bin/metric cmd/huker-metrics.go

test:
	go get github.com/go-playground/overalls
	go get github.com/mattn/goveralls
	overalls -project=github.com/openinx/huker -covermode=count -ignore='.git,vendor'

travis-test: test
	goveralls -coverprofile=overalls.coverprofile -service=travis-ci

release: build
	@rm -rf release
	@mkdir -p release/$(HUKER_VERSION)
	@cp -R bin conf ansible site release/$(HUKER_VERSION)
	@cd release; tar czvf $(HUKER_VERSION).tar.gz $(HUKER_VERSION) >/dev/null 2>&1
	@echo "Huker release package: release/$(HUKER_VERSION).tar.gz"

clean:
	rm -rf bin/* log/* release/* *.coverprofile
