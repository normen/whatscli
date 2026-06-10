# Simple Makefile for go

build:
	go build

clean:
	go clean

run:
	go run .

install:
	go install .

get:
	go get

update:
	go get -u

# remonta dist/ (binarios Linux/Windows + atalhos/instaladores + zips)
dist:
	./make-dist.sh

release:
	./release.sh

.PHONY: build clean run install get update dist release
