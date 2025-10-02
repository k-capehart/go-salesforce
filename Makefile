all: tidy build

tidy:
	go mod tidy

generate:
	go generate ./...

build: generate
	go build -v ./...

install-tools:
	go install github.com/segmentio/golines@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install mvdan.cc/gofumpt@latest
	
fmt:
	gofumpt -w .
	golines -w .
	goimports -w .

lint:
	@golangci-lint run

mod-upgrade:
	go get -u ./...

test:
	go test -cover

test-output:
	go test -v -coverprofile cover.out && go tool cover -html cover.out -o cover.html && open cover.html

.PHONY: all tidy generate build install-tools fmt lint mod-upgrade test test-output