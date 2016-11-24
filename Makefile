all: install test
.PHONY: all

install:
	@echo "[+] installing dependencies"
	@go get -t -v ./...
.PHONY: install

test:
	@echo "[+] testing"
	@go test -v ./...
.PHONY: test

int-test: install
	$(eval tmp := $(shell pwd)"/tmp")

	@echo "[+] intergration testing"

	@rm -rf $(tmp)
	@mkdir $(tmp)

	@go build -v ./bin/ec-proxy
	@mv ec-proxy $(tmp)

	@env EC_PROXY_PLACE=$(tmp) EC_WITHOUT_SPINNER=true INT=true go test -v ./bin/ec -timeout 90m
	@rm -rf $(tmp)
.PHONY: int-test

build:
	@echo "[+] building"
	@go get github.com/mitchellh/gox
	@rm -rf ec_* ec-proxy_*
	@gox -osarch="darwin/amd64 linux/amd64" ./...
.PHONY: build

tag:
	$(eval version := $(shell go run bin/ec/main.go version))
	@echo "[+] tagging"
	@git tag v$(version) -a -m "Release v$(version)"
.PHONY: tag

release:
	@echo "[+] releasing"
	@$(MAKE) test
	@$(MAKE) build
	@$(MAKE) tag
	@echo "[+] complete"
.PHONY: release
