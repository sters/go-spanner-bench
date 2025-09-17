
export GOBIN := $(PWD)/bin
export PATH := $(GOBIN):$(PATH)
export SPANNER_EMULATOR_HOST := localhost:9010

.PHONY: init
init:
	SPANNER_EMULATOR_HOST=$(SPANNER_EMULATOR_HOST) go run cmd/init/main.go \
		--project=test-project \
		--instance=test-instance \
		--database=test-database

.PHONY: bench-all
bench-all: init
	@echo "Benchmarking all table types..."
	@mkdir -p logs results
	@for table in base fulltext substring ngrams; do \
		echo "\n=== Benchmarking $$table ===" ; \
		SPANNER_EMULATOR_HOST=$(SPANNER_EMULATOR_HOST) go run cmd/query-bench/main.go \
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
	SPANNER_EMULATOR_HOST=$(SPANNER_EMULATOR_HOST) go run cmd/create-data-bench/main.go \
		--project=test-project \
		--instance=test-instance \
		--database=test-database \
		--bq-project=dummy \
		--bq-query="SELECT 'test data'" \
		--batch-size=10 \
		--max-rows=100 || true
	@echo "Running query benchmark..."
	SPANNER_EMULATOR_HOST=$(SPANNER_EMULATOR_HOST) go run cmd/query-bench/main.go \
		--project=test-project \
		--instance=test-instance \
		--database=test-database \
		--table=fulltext \
		--search=test \
		--query-type=search \
		--limit=10 \
		--iterations=10 \
		--goroutines=2
