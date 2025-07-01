all: tidy build

tidy:
	go mod tidy
  # go mod vendor

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
	go test ./... -v

