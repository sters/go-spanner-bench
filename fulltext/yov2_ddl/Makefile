
ROOT_DIR = $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
export GOBIN := $(ROOT_DIR)/bin
export PATH := $(GOBIN):$(PATH)

DDL_FILE := ../config/schema.sql

.PHONY: setup
setup:
	@go install go.mercari.io/yo/v2@HEAD

.PHONY: generate
generate: clean-models
	@$(GOBIN)/yo --version
	@echo "Generating Spanner models from DDL..."
	@echo "DDL file: $(DDL_FILE)"
	@$(GOBIN)/yo generate $(DDL_FILE) \
		--from-ddl \
		-o . \
		-p models
	@echo "Model generation from DDL complete!"

.PHONY: clean-models
clean-models:
	@echo "Cleaning generated files..."
	@rm -f *.yo.go
	@echo "Clean complete!"
