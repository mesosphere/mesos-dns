DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

default: all

all: deps format build

deps:
	@echo "--> Installing build dependencies"
	@go get -d -v ./... $(DEPS)

updatedeps: deps
	@echo "--> Updating build dependencies"
	@go get -d -f -u ./... $(DEPS)

format: deps
	@echo "--> Running go fmt"
	@go fmt ./...

build: deps
	@echo "--> Building mesos-dns"
	@go build -o mesos-dns

test: deps
	@go test ./...

testrace: deps
	@go test -race ./...

clean:
	@go clean
