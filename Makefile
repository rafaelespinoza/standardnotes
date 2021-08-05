GO ?= go
BIN_DIR=bin

deps:
	$(GO) mod tidy -v && $(GO) mod vendor

build:
	mkdir -pv $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/standardnotes .

.PHONY: test
# Specify packages to test with P variable. Example:
# make test P='entity repo'
#
# Specify test flags with T variable. Example:
# make test T='-v -count=1 -failfast'
test: P ?= ...
test: pkgpath=$(foreach pkg,$(P),$(shell echo ./internal/$(pkg)))
test: DB_DSN=$(DB_DSN_TEST)
test:
	$(GO) test $(pkgpath) $(T)
