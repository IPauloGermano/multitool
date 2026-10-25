BINARY_NAME=multitool
DIST_DIR=dist
LDFLAGS=-s -w -X 'github.com/IPauloGermano/multitool/cmd.Version=2.3.1' -X 'github.com/IPauloGermano/multitool/cmd.BuildDate=$(shell date +%Y-%m-%d)'

.PHONY: all test build build-linux build-windows build-all clean

all: test build

test:
	go test -v ./...

build:
	mkdir -p $(DIST_DIR)
	rm -f $(DIST_DIR)/$(BINARY_NAME)
	go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) main.go

build-linux:
	mkdir -p $(DIST_DIR)
	rm -f $(DIST_DIR)/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-linux-*
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	cp $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(DIST_DIR)/$(BINARY_NAME)
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 main.go

build-windows:
	mkdir -p $(DIST_DIR)
	rm -f $(DIST_DIR)/$(BINARY_NAME).exe $(DIST_DIR)/$(BINARY_NAME)-arm64.exe
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME).exe main.go
	GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-arm64.exe main.go

build-all: build-linux build-windows
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 main.go

clean:
	rm -rf $(DIST_DIR)
