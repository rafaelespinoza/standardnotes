IMG_NAME=rafaele/standardfile
IMG_TAG=latest
LOCAL_DATA_DIR=$(HOME)/.sf

test:
	go test $(ARGS) ./...

build:
	go build ./...

install:
	GO111MODULE=on go mod download && go mod verify && go install -i -v .

docker-build:
	docker build \
		--build-arg BUILD_TIME=$(shell date +%FT%T%z) \
		--build-arg GIT_BRANCH=$(shell git branch --show-current) \
		--build-arg SF_VERSION=$(shell git rev-parse --short HEAD) \
		-t $(IMG_NAME) .

docker-run:
	mkdir -p $(LOCAL_DATA_DIR)
	docker run -d -v $(LOCAL_DATA_DIR):/data -p 8888:8888 $(IMG_NAME):$(IMG_TAG)

docker-stop:
	docker stop $(shell docker ps -f name="\bstandardfile\b" -q)

docker-up:
	docker-compose up

docker-down:
	docker-compose down
