.PHONY: all build-linux build-windows build-darwin-amd64 build-darwin-arm64

BINARY_NAME=kubectl-view_labels
VERSION=v0.0.1

all: build-linux build-windows build-darwin-amd64 build-darwin-arm64

build-linux:
	GOOS=linux GOARCH=amd64 go build -o build/$(BINARY_NAME)-$(VERSION)-linux-amd64

build-windows:
	GOOS=windows GOARCH=amd64 go build -o build/$(BINARY_NAME)-$(VERSION)-windows-amd64.exe

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o build/$(BINARY_NAME)-$(VERSION)-darwin-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o build/$(BINARY_NAME)-$(VERSION)-darwin-arm64
