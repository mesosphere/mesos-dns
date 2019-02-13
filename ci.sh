#!/usr/bin/env bash

set -e

# environment
export PATH="${PATH}:${GOPATH}/bin"
readonly PROJECT_DIR="${GOPATH}/src/github.com/mesosphere/mesos-dns"
readonly ARTIFACT_DIR="${WORKSPACE}/target"
readonly TEST_REPORTS="${WORKSPACE}/test_results"
export GO15VENDOREXPERIMENT=1

go version
go get github.com/kardianos/govendor

cd $PROJECT_DIR

go get github.com/mitchellh/gox
go get github.com/alecthomas/gometalinter
go get github.com/axw/gocov/gocov # https://github.com/golang/go/issues/6909
go get github.com/mattn/goveralls
go get github.com/jstemmer/go-junit-report

# only import the key if it is not already known, otherwise we get an error that halts the build
gpg --list-keys|grep -q -e BD292F47 || \
  gpg --yes --batch --import build/private.key

go install ./...
go test -i ./...

gox -parallel=1 -arch=amd64 -os="linux darwin windows" \
  -output="${ARTIFACT_DIR}/{{.Dir}}-${PACKAGE_VERSION}-{{.OS}}-{{.Arch}}" \
  -ldflags="-X main.Version=${PACKAGE_VERSION}"

# run tests
gometalinter --install
gometalinter \
  --vendor --concurrency=1 --cyclo-over=12 --tests \
  --exclude='TLS InsecureSkipVerify may be true.' \
  --exclude='Use of unsafe calls should be audited' \
  --deadline=300s ./...

gocov test ./... -short -timeout=10m > cov.json

mkdir -p $TEST_REPORTS/junit && go test -v -timeout=10m ./... | \
  go-junit-report > $TEST_REPORTS/junit/alltests.xml

go test -v -short -race -timeout=10m ./...
