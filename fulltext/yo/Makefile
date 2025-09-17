ROOT_DIR = $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
export GOBIN := $(ROOT_DIR)/bin
export PATH := $(GOBIN):$(PATH)
export SPANNER_EMULATOR_HOST := localhost:9010

PROJECT_ID := test-project
INSTANCE_ID := test-instance
DATABASE_ID := test-database

.PHONY: setup
setup:
	@go install go.mercari.io/yo@HEAD

.PHONY: generate
generate: clean-models
	@$(GOBIN)/yo --version
	@echo "Generating Spanner models..."
	@SPANNER_EMULATOR_HOST=localhost:9010 \
		$(GOBIN)/yo \
		$(PROJECT_ID) \
		$(INSTANCE_ID) \
		$(DATABASE_ID) \
		--out . \
		--package models \
		--template-path templates
	@echo "Model generation complete!"

.PHONY: clean-models
clean-models:
	@echo "Cleaning generated files..."
	@rm -f *.yo.go
	@echo "Clean complete!"
