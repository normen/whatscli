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

release:
	./release.sh
