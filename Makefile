BINARY_NAME=multitool
BUILD_DIR=bin
LDFLAGS=-s -w -X 'github.com/IPauloGermano/multitool/cmd.Version=2.0.0' -X 'github.com/IPauloGermano/multitool/cmd.BuildDate=$(shell date +%Y-%m-%d)'

.PHONY: all test build build-linux build-windows build-all clean

all: test build

test:
	go test -v ./...

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) main.go

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-arm64 main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME).exe main.go
	GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-arm64.exe main.go

build-all: build-linux build-windows
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-arm64 main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-amd64 main.go

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)*.exe $(BINARY_NAME)-linux-* $(BINARY_NAME)-darwin-*
