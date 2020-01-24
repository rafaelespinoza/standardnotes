test:
	go test $(ARGS) ./...

build:
	go build ./...

install:
	GO111MODULE=on go mod download && go mod verify && go install -i -v .
