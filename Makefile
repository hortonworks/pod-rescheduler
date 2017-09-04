BINARY=pod-rescheduler

VERSION=0.2
BUILD_TIME=$(shell date +%FT%T)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

deps:
	go get -u github.com/kardianos/govendor

format:
	@gofmt -w ${GOFILES_NOVENDOR}

build: format build-darwin build-linux

build-darwin:
	GOOS=darwin CGO_ENABLED=0 go build -a ${LDFLAGS} -o build/Darwin/${BINARY} main.go

build-linux:
	GOOS=linux CGO_ENABLED=0 go build -a ${LDFLAGS} -o build/Linux/${BINARY} main.go

.DEFAULT_GOAL := build

.PHONY: build