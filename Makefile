test:
	go test ./...

build:
	go build -o bin/fgamanager .

all: test build