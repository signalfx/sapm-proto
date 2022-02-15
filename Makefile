ALL_SRC := $(shell find . -name '*.go' \
                                -not -path './internal/tools/*' \
                                -not -path './gen/*' \
                                -type f | sort)

# ALL_PKGS is the list of all packages where ALL_SRC files reside.
ALL_PKGS := $(shell go list $(sort $(dir $(ALL_SRC))))

# All source code and documents. Used in spell check.
ALL_DOCS := $(shell find . -name '*.md' -type f | sort)

ALL_GO_MOD_DIRS := $(shell find . -type f -name 'go.mod' -exec dirname {} \; | sort)

# Function to execute a command. Note the empty line before endef to make sure each command
# gets executed separately instead of concatenated with previous one.
# Accepts command to execute as first parameter.
define exec-command
$(1)

endef

GO_ACC=go-acc
GOOS=$(shell go env GOOS)
ADDLICENCESE=addlicense
MISSPELL=misspell -error
MISSPELL_CORRECTION=misspell -w
IMPI=impi

.PHONY: all
all: check
	$(MAKE) generate-sapm
	$(MAKE) generate-otlp
	$(MAKE) test

OTEL_DOCKER_PROTOBUF ?= otel/build-protobuf:0.10.0

SAPM_PROTO_INCLUDES := -I/usr/include/github.com/gogo/protobuf
SAPM_PROTOC := docker run --rm -u ${shell id -u} -v${PWD}:${PWD} -w${PWD} ${OTEL_DOCKER_PROTOBUF} --proto_path=${PWD}
# Target directory to write generated files to.
SAPM_TARGET_GEN_DIR=./gen

.PHONY: generate-sapm
generate-sapm:
	git submodule update --init

	mkdir -p $(SAPM_TARGET_GEN_DIR)
	$(SAPM_PROTOC) $(SAPM_PROTO_INCLUDES) --gogo_out=,Mjaeger-idl/proto/api_v2/model.proto=github.com/jaegertracing/jaeger/model:$(SAPM_TARGET_GEN_DIR) proto/sapm.proto

	@echo Move generated code to target directory.
	cp -R $(SAPM_TARGET_GEN_DIR)/proto/* $(SAPM_TARGET_GEN_DIR)
	rm -fr $(SAPM_TARGET_GEN_DIR)/proto

.PHONY: check
check: addlicense lint misspell

.PHONY: gomoddownload
gomoddownload:
	go mod download

.PHONY: test
test:
	go test ./...

.PHONY: addlicense
addlicense:
	@ADDLICENCESEOUT=`$(ADDLICENCESE) -y 2019 -c 'Splunk, Inc.' $(ALL_SRC) 2>&1`; \
		if [ "$$ADDLICENCESEOUT" ]; then \
			echo "$(ADDLICENCESE) FAILED => add License errors:\n"; \
			echo "$$ADDLICENCESEOUT\n"; \
			exit 1; \
		else \
			echo "Add License finished successfully"; \
		fi

.PHONY: lint
lint:
	$(foreach dir,$(ALL_GO_MOD_DIRS),$(call exec-command,cd $(dir) && golangci-lint run --fix && golangci-lint run))
	$(foreach dir,$(ALL_GO_MOD_DIRS),$(call exec-command,cd $(dir) && go mod tidy))

.PHONY: misspell
misspell:
	$(MISSPELL) $(ALL_DOCS)

.PHONY: misspell-correction
misspell-correction:
	$(MISSPELL_CORRECTION) $(ALL_DOCS)

.PHONY: impi
impi:
	@$(IMPI) --local github.com/signalfx/sapm-proto --scheme stdThirdPartyLocal $(ALL_SRC)

.PHONY: test-with-cover
test-with-cover:
	$(GO_ACC) $(ALL_PKGS)
	go tool cover -html=coverage.txt -o coverage.html

TOOLS_MOD_DIR := ./internal/tools
.PHONY: install-tools
install-tools:
	cd $(TOOLS_MOD_DIR) && go install github.com/client9/misspell/cmd/misspell
	cd $(TOOLS_MOD_DIR) && go install github.com/golangci/golangci-lint/cmd/golangci-lint
	cd $(TOOLS_MOD_DIR) && go install github.com/google/addlicense
	cd $(TOOLS_MOD_DIR) && go install github.com/ory/go-acc
	cd $(TOOLS_MOD_DIR) && go install github.com/pavius/impi/cmd/impi
