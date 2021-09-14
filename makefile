ALL_SRC := $(shell find . -name '*.go' \
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
STATICCHECK=staticcheck
IMPI=impi

.PHONY: all
all: check
	$(MAKE) generate-sapm
	$(MAKE) generate-otlp
	$(MAKE) test

JAEGER_DOCKER_PROTOBUF=jaegertracing/protobuf:0.2.0
SAPM_PROTO_INCLUDES := -I/usr/include/github.com/gogo/protobuf
SAPM_PROTOC := docker run --rm -u ${shell id -u} -v${PWD}:${PWD} -w${PWD} ${JAEGER_DOCKER_PROTOBUF} --proto_path=${PWD}
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

# Target directory to write generated files to.
OTLP_GEN_GO_DIR=./gen/otlp

# The source directory for OTLP ProtoBufs.
OTLP_PROTO_SRC_DIR=opentelemetry-proto

# Intermediate directory used during generation.
OTLP_PROTO_INTERMEDIATE_DIR=$(OTLP_GEN_GO_DIR)/.patched-otlp-proto

# Go package name to use for generated files.
OTLP_PROTO_PACKAGE=github.com/signalfx/sapm-proto/$(OTLP_GEN_GO_DIR)

# Find all .proto files.
OTLP_PROTO_FILES := opentelemetry/proto/common/v1/common.proto opentelemetry/proto/resource/v1/resource.proto opentelemetry/proto/trace/v1/trace.proto opentelemetry/proto/collector/trace/v1/trace_service.proto

OTEL_DOCKER_PROTOBUF ?= otel/build-protobuf:0.2.1
OTLP_PROTOC := docker run --rm -u ${shell id -u} -v${PWD}:${PWD} -w${PWD}/$(OTLP_PROTO_INTERMEDIATE_DIR) ${OTEL_DOCKER_PROTOBUF} --proto_path=${PWD}/$(OTLP_PROTO_INTERMEDIATE_DIR)
PROTO_INCLUDES := -I/usr/include/github.com/gogo/protobuf

.PHONY: generate-otlp
generate-otlp:
	git submodule update --init

	@echo Delete intermediate directory.
	@rm -rf $(OTLP_PROTO_INTERMEDIATE_DIR)

	@echo Delete target directory.
	@rm -rf $(OTLP_GEN_GO_DIR)

	@echo $(OTLP_PROTO_FILES)

	@echo Copy .proto file to intermediate directory.
	$(foreach file,$(OTLP_PROTO_FILES),$(call exec-command, mkdir -p $(OTLP_PROTO_INTERMEDIATE_DIR)/$(shell dirname "$(file)") && cp -R $(OTLP_PROTO_SRC_DIR)/$(file) $(OTLP_PROTO_INTERMEDIATE_DIR)/$(file)))

	@echo Modify them in the intermediate directory.
	$(foreach file,$(OTLP_PROTO_FILES),$(call exec-command,sed 's+github.com/open-telemetry/opentelemetry-proto/gen/go/+github.com/signalfx/sapm-proto/gen/otlp/+g' $(OTLP_PROTO_SRC_DIR)/$(file) > $(OTLP_PROTO_INTERMEDIATE_DIR)/$(file)))

	@echo Generate Go code from .proto files in intermediate directory.
	$(foreach file,$(OTLP_PROTO_FILES),$(call exec-command, $(OTLP_PROTOC) $(PROTO_INCLUDES) --gogofaster_out=plugins=grpc:./ $(file)))

	@echo Move generated code to target directory.
	mkdir -p $(OTLP_GEN_GO_DIR)
	cp -R $(OTLP_PROTO_INTERMEDIATE_DIR)/$(OTLP_PROTO_PACKAGE)/* $(OTLP_GEN_GO_DIR)/
	rm -rf $(OTLP_PROTO_INTERMEDIATE_DIR)

.PHONY: check
check: addlicense lint misspell staticcheck

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

.PHONY: staticcheck
staticcheck:
	$(STATICCHECK) $(ALL_PKGS)

.PHONY: impi
impi:
	@$(IMPI) --local github.com/signalfx/sapm-proto --scheme stdThirdPartyLocal $(ALL_SRC)

.PHONY: test-with-cover
test-with-cover:
	@echo pre-compiling tests
	go test -i $(ALL_PKGS)
	$(GO_ACC) $(ALL_PKGS)
	go tool cover -html=coverage.txt -o coverage.html

.PHONY: install-tools
install-tools:
	GO111MODULE=on go install \
	  github.com/google/addlicense \
 	  github.com/golangci/golangci-lint/cmd/golangci-lint \
	  github.com/client9/misspell/cmd/misspell \
	  honnef.co/go/tools/cmd/staticcheck \
	  github.com/ory/go-acc \
	  github.com/pavius/impi/cmd/impi
