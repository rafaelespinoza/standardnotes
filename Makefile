BUILD_DIR=/tmp/standardnotes
IMG_NAME=rafaele/standardnotes_api
IMG_TAG=latest

_DB_BASENAME=standardnotes
DB_DEV_USER=$(addsuffix _dev, $(_DB_BASENAME))
DB_DEV_HOST=localhost
DB_DEV_NAME=$(DB_DEV_USER)
DB_TEST_USER=$(addsuffix _test, $(_DB_BASENAME))
DB_TEST_HOST=localhost
DB_TEST_NAME=$(DB_TEST_USER)
DB_MIGRATION_TOOL=./godfish
DB_MIGRATION_FILES=./db/migrations

_ENVFILE_BASENAME=./.env
ENVFILE_DEV=$(addsuffix -dev, $(_ENVFILE_BASENAME))
ENVFILE_TEST=$(addsuffix -test, $(_ENVFILE_BASENAME))

test: db-test
	. $(ENVFILE_TEST) && $(shell sed 's/=.*//' $(ENVFILE_TEST)) && go test ./... $(ARGS)

build:
	go build ./...

install:
	GO111MODULE=on go mod download && go mod verify && go install -i -v .

#
# db
#

db-dev: db-dev-teardown db-dev-setup db-dev-migrate
db-dev-teardown:
	mysql -u $(DB_DEV_USER) -h $(DB_DEV_HOST) -e "DROP DATABASE IF EXISTS ${DB_DEV_NAME}"
db-dev-setup:
	mysql -u $(DB_DEV_USER) -h $(DB_DEV_HOST) -e "CREATE DATABASE IF NOT EXISTS ${DB_DEV_NAME}"
db-dev-migrate: db-migration-tool
	. $(ENVFILE_DEV) && $(shell sed 's/=.*//' $(ENVFILE_DEV)) \
		&& $(DB_MIGRATION_TOOL) -files $(DB_MIGRATION_FILES) migrate

db-test: db-test-teardown db-test-setup db-test-migrate
db-test-teardown:
	mysql -u $(DB_TEST_USER) -h $(DB_TEST_HOST) -e "DROP DATABASE IF EXISTS ${DB_TEST_NAME}"
db-test-setup:
	mysql -u $(DB_TEST_USER) -h $(DB_TEST_HOST) -e "CREATE DATABASE IF NOT EXISTS ${DB_TEST_NAME}"
db-test-migrate: db-migration-tool
	. $(ENVFILE_TEST) && $(shell sed 's/=.*//' $(ENVFILE_TEST)) \
		&& $(DB_MIGRATION_TOOL) -files $(DB_MIGRATION_FILES) migrate

db-migration-tool:
	GOOS=linux GOARCH=amd64 go build -o $(DB_MIGRATION_TOOL) -i -v \
		 github.com/rafaelespinoza/godfish/mysql/godfish

#
# docker
#

docker-build:
	mkdir -p $(BUILD_DIR)
	cp --parents $(shell git ls-files | grep -v _test.go) $(BUILD_DIR)
	docker build -t $(IMG_NAME) .

docker-stop:
	docker stop $(shell docker ps -f ancestor="$(IMG_NAME)" -q)

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

#
# env
#

define ENVFILE_TEMPLATE
# api
export CONTAINER_PORT=
export HOST_PORT=
export SECRET_KEY_BASE=

# db
export DB_HOST=
export DB_NAME=standardnotes_dev
export DB_PASSWORD=
export DB_PORT=3306
export DB_ROOT_PASSWORD=
export DB_USER=standardnotes_dev
endef

envfile: envfile-dev envfile-test

envfile-dev:
ifeq ("","$(wildcard $ENVFILE_DEV)")
	$(file > $(ENVFILE_DEV),$(ENVFILE_TEMPLATE))
endif

envfile-test:
ifeq ("","$(wildcard $ENVFILE_TEST)")
	$(file > $(ENVFILE_TEST),$(ENVFILE_TEMPLATE))
	sed -i 's/dev/test/g' $(ENVFILE_TEST)
endif
