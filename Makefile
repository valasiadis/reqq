SRCS = $(shell git ls-files '*.go')

BIN_NAME := reqq

# Default build target
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
GOGC ?=

CONTAINER_BUILD_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: default
#? default: Runs `make build-binary` and `make build-image`
default: build-binary build-image

#? dist: Create the "dist" directory
dist:
	mkdir -p dist/

.PHONY: build-image
#? build-image: Build an OCI image
build-image:
	buildah build -t reqq .

.PHONY: build-binary
#? build-image: Build the application
build-binary:
	go build -o ./dist/$(BIN_NAME) $(SRCS)

.PHONY: fmt
#? fmt: Format the code
fmt:
	gofmt -s -l -w $(SRCS)

.PHONY: clean
#? clean: Clean up build directories and temporary files
clean:
	rm -rf ./dist/

.PHONY: help
#? help: Get more info on make commands
help: Makefile
	@echo " Choose a command run in reqq:"
	@sed -n 's/^#?//p' $< | column -t -s ':' |  sort | sed -e 's/^/ /'
