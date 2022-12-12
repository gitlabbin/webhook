OS = linux
ARCHS = amd64

.DEFAULT_GOAL := help

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

#all: build release release-windows
all: fmt lint vet build release

build: deps ## Build the project
	go build

release: clean deps ## Generate releases for unix systems
	@for arch in $(ARCHS);\
	do \
		for os in $(OS);\
		do \
			echo "Building $$os-$$arch"; \
			mkdir -p build/webhook-$$os-$$arch/; \
			GOOS=$$os GOARCH=$$arch go build -o build/webhook-$$os-$$arch/webhook; \
			tar cz -C build -f build/webhook-$$os-$$arch.tar.gz webhook-$$os-$$arch; \
		done \
	done
	cp build/webhook-linux-amd64/webhook ~/workspace/icc/cicd-hooks/docker-webhook/bin

release-windows: clean deps ## Generate release for windows
	@for arch in $(ARCHS);\
	do \
		echo "Building windows-$$arch"; \
		mkdir -p build/webhook-windows-$$arch/; \
		GOOS=windows GOARCH=$$arch go build -o build/webhook-windows-$$arch/webhook.exe; \
		tar cz -C build -f build/webhook-windows-$$arch.tar.gz webhook-windows-$$arch; \
	done

test: deps ## Execute tests
	go test ./...

deps: ## Install dependencies using go get
	go get -d -v -t ./...

clean: ## Remove building artifacts
	rm -rf build
	rm -f webhook

tools:
	go get golang.org/x/tools/cmd/goimports
	go get github.com/kisielk/errcheck
	go get github.com/golang/lint/golint
	go get github.com/axw/gocov/gocov
	go get github.com/matm/gocov-html
	go get github.com/tools/godep
	go get github.com/mitchellh/gox

vet:
	go vet -v ./...

errors:
	errcheck -ignoretests -blank ./...

lint:
	golint ./...

imports:
	goimports -l -w .

fmt: imports
	go fmt ./...
