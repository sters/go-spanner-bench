ROOT_DIR = $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
export GOBIN := $(ROOT_DIR)/bin
export PATH := $(GOBIN):$(PATH)
export SPANNER_EMULATOR_HOST := localhost:9010

PROJECT_ID := test-project
INSTANCE_ID := test-instance
DATABASE_ID := test-database

SPANNER_DSN := spanner://projects/$(PROJECT_ID)/instances/$(INSTANCE_ID)/databases/$(DATABASE_ID)
SCHEMA_FILE := ../cmd/init/schema.sql

.PHONY: setup
setup:
	@go install github.com/daichirata/hammer@HEAD

.PHONY: export
export: setup
	@echo "Exporting current schema from Spanner emulator..."
	@$(GOBIN)/hammer export $(SPANNER_DSN) > current_schema.sql
	@echo "Schema exported to current_schema.sql"

.PHONY: diff
diff: setup export
	@echo "Comparing schema.sql with Spanner emulator..."
	@echo "-----"
	$(GOBIN)/hammer diff $(SCHEMA_FILE) $(SPANNER_DSN)
	@echo "-----"
	$(GOBIN)/hammer diff $(SPANNER_DSN) $(SCHEMA_FILE)
	@echo "-----"

.PHONY: export-and-diff
export-and-diff: export
	diff -u current_schema.sql $(SCHEMA_FILE) || true
