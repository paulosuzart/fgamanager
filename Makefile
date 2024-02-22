test:
	go test manager/...

build:
	go build -o bin/manager manager

all: test build