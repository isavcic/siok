all:
	go get -d
	go build -ldflags="-s -w"
