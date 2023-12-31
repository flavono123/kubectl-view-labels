.PHONY: all build-linux build-windows build-darwin-amd64 build-darwin-arm64

BINARY_NAME=kubectl-view_labels
VERSION=v0.1.0

all: build-linux build-windows build-darwin-amd64 build-darwin-arm64

build-linux:
	GOOS=linux GOARCH=amd64 go build -o build/$(BINARY_NAME)_$(VERSION)_linux_amd64
	tar czvf build/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz -C build $(BINARY_NAME)_$(VERSION)_linux_amd64 -C .. LICENSE
	shasum -a 256 build/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz | awk '{print $$1}' > build/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz.sha256

build-windows:
	GOOS=windows GOARCH=amd64 go build -o build/$(BINARY_NAME)_$(VERSION)_windows_amd64.exe
	zip build/$(BINARY_NAME)_$(VERSION)_windows_amd64.zip build/$(BINARY_NAME)_$(VERSION)_windows_amd64.exe LICENSE
	shasum -a 256 build/$(BINARY_NAME)_$(VERSION)_windows_amd64.zip | awk '{print $$1}' > build/$(BINARY_NAME)_$(VERSION)_windows_amd64.zip.sha256

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o build/$(BINARY_NAME)_$(VERSION)_darwin_amd64
	tar czvf build/$(BINARY_NAME)_$(VERSION)_darwin_amd64.tar.gz -C build $(BINARY_NAME)_$(VERSION)_darwin_amd64 -C .. LICENSE
	shasum -a 256 build/$(BINARY_NAME)_$(VERSION)_darwin_amd64.tar.gz | awk '{print $$1}' > build/$(BINARY_NAME)_$(VERSION)_darwin_amd64.tar.gz.sha256

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o build/$(BINARY_NAME)_$(VERSION)_darwin_arm64
	tar czvf build/$(BINARY_NAME)_$(VERSION)_darwin_arm64.tar.gz -C build $(BINARY_NAME)_$(VERSION)_darwin_arm64 -C .. LICENSE
	shasum -a 256 build/$(BINARY_NAME)_$(VERSION)_darwin_arm64.tar.gz | awk '{print $$1}' > build/$(BINARY_NAME)_$(VERSION)_darwin_arm64.tar.gz.sha256
