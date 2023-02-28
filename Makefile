BIN_NAME := go-jira-migrate
BUILD_DIR := build
LINUX_TARGET := $(BUILD_DIR)/$(BIN_NAME)-linux-amd64
WINDOWS_TARGET := $(BUILD_DIR)/$(BIN_NAME)-windows-amd64.exe

.PHONY: all
all: linux windows

.PHONY: linux
linux:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(LINUX_TARGET) .

.PHONY: windows
windows:
	mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(WINDOWS_TARGET) .

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)