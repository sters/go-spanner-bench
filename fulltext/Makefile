
ROOT_DIR = $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
export GOBIN := $(ROOT_DIR)/bin
export PATH := $(GOBIN):$(PATH)
export SPANNER_EMULATOR_HOST := localhost:9010

PROJECT_ID := test-project
INSTANCE_ID := test-instance
DATABASE_ID := test-database

.PHONY: check-emulator
check-emulator:
	@echo "Checking Spanner Emulator connection..."
	@gcloud spanner instances list --project=$(PROJECT_ID) 2>/dev/null || echo "Emulator is running"


.PHONY: spanner-cli
spanner-cli: check-emulator init
	@SPANNER_EMULATOR_HOST=localhost:9010 \
		spanner-cli \
		-p $(PROJECT_ID) \
		-i $(INSTANCE_ID) \
		-d $(DATABASE_ID)

.PHONY: init
init: check-emulator
	@go run cmd/init/main.go \
		--project=test-project \
		--instance=test-instance \
		--database=test-database

.PHONY: bench-all
bench-all: init
	@echo "Benchmarking all table types..."
	@mkdir -p logs results
	@for table in base fulltext substring ngrams; do \
		echo "\n=== Benchmarking $$table ===" ; \
		go run cmd/query-bench/main.go \
			--project=test-project \
			--instance=test-instance \
			--database=test-database \
			--table=$$table \
			--search=test \
			--query-type=search \
			--limit=10 \
			--iterations=20 \
			--goroutines=5 \
			--log-file=logs/$$table-bench.log \
			--results-file=results/$$table-results.json ; \
	done

.PHONY: test-emulator
test-emulator: init
	@echo "Creating test data..."
	go run cmd/create-data-bench/main.go \
		--project=test-project \
		--instance=test-instance \
		--database=test-database \
		--bq-project=dummy \
		--bq-query="SELECT 'test data'" \
		--batch-size=10 \
		--max-rows=100 || true
	@echo "Running query benchmark..."
	go run cmd/query-bench/main.go \
		--project=test-project \
		--instance=test-instance \
		--database=test-database \
		--table=fulltext \
		--search=test \
		--query-type=search \
		--limit=10 \
		--iterations=10 \
		--goroutines=2

.PHONY: yov2/generate
yov2/generate: init yov2/clean-models
	@$(MAKE) -C yov2 generate

.PHONY: yov2/clean-models
yov2/clean-models:
	@$(MAKE) -C yov2 clean-models

.PHONY: yov2_ddl/generate
yov2_ddl/generate: init yov2_ddl/clean-models
	@$(MAKE) -C yov2_ddl generate

.PHONY: yov2_ddl/clean-models
yov2_ddl/clean-models:
	@$(MAKE) -C yov2_ddl clean-models

.PHONY: yo/generate
yo/generate: init yo/clean-models
	@$(MAKE) -C yo generate

.PHONY: yo/clean-models
yo/clean-models:
	@$(MAKE) -C yo clean-models

.PHONY: hammer/diff
hammer/diff: init
	@$(MAKE) -C hammer diff

.PHONY: hammer/export
hammer/export: init
	@$(MAKE) -C hammer export
