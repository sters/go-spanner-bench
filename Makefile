
export GOBIN := $(PWD)/bin
export PATH := $(GOBIN):$(PATH)
export SPANNER_EMULATOR_HOST := localhost:9010

.PHONY: emulator-up
emulator-up:
	@docker-compose up -d

.PHONY: emulator-down
emulator-down:
	docker-compose down

.PHONY: lint
lint:
	go tool golangci-lint run -v ./...

.PHONY: lint-fix
lint-fix:
	go tool golangci-lint run --fix -v ./...

.PHONY: test
test:
	go test -v -race ./...

.PHONY: cover
cover:
	go test -v -race -coverpkg=./... -coverprofile=coverage.out ./...

.PHONY: tidy
tidy:
	go mod tidy


.PHONY: fulltext/spanner-cli
fulltext/spanner-cli: emulator-up
	@$(MAKE) -C fulltext spanner-cli

.PHONY: fulltext/init
fulltext/init: emulator-up
	@$(MAKE) -C fulltext init

.PHONY: fulltext/bench-all
fulltext/bench-all: emulator-up
	@$(MAKE) -C fulltext bench-all

.PHONY: fulltext/test-emulator
fulltext/test-emulator: emulator-up
	@$(MAKE) -C fulltext test-emulator

.PHONY: fulltext/yov2/generate
fulltext/yov2/generate: emulator-up
	@$(MAKE) -C fulltext yov2/generate

.PHONY: fulltext/yov2_ddl/generate
fulltext/yov2_ddl/generate: emulator-up
	@$(MAKE) -C fulltext yov2_ddl/generate

.PHONY: fulltext/yo/generate
fulltext/yo/generate: emulator-up
	@$(MAKE) -C fulltext yo/generate

.PHONY: fulltext/hammer/diff
fulltext/hammer/diff: emulator-up
	@$(MAKE) -C fulltext hammer/diff

.PHONY: fulltext/hammer/export
fulltext/hammer/export: emulator-up
	@$(MAKE) -C fulltext hammer/export
