DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

default: all

all: restoredeps test build

restoredeps:
	@echo "--> Restoring build dependencies"
	@godep restore

savedeps: 
	@echo "--> Saving build dependencies"
	@godep save

updatedeps: 
	@echo "--> Updating build dependencies"
	@godep update ${ARGS}

format: 
	@echo "--> Running go fmt"
	@godep go fmt ./...

vet: 
	@echo "--> Running go vet"
	@godep go vet ./...

build: 
	@echo "--> Building mesos-dns"
	@godep go build -o mesos-dns

test: 
	@echo "--> Testing mesos-dns"
	@godep go test ./...

testrace: 
	@godep go test -race ./...

clean:
	@echo "--> Cleaning mesos-dns"
	@godep go clean
