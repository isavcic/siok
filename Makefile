all:
	go get -d
	go build -ldflags="-s -w -X main.version=$(shell git rev-parse --short HEAD)"
