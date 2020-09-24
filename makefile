ALL_SRC := $(shell find . -name '*.go' \
                                -not -path './gen/*' \
                                -not -path './vendor/*' \
                                -type f | sort)

# All source code and documents. Used in spell check.
ALL_SRC_AND_DOC := $(shell find . \( -name "*.md" -o -name "*.go" -o -name "*.yaml" \) \
                                -not -path './gen/*' \
                                -not -path './vendor/*' \
                                -type f | sort)

# ALL_PKGS is used with 'go cover'
ALL_PKGS := $(shell go list $(sort $(dir $(ALL_SRC))))

PROTO_PACKAGE_PATH?=./gen/
GOTEST_OPT?= -race -timeout 30s
GOTEST_OPT_WITH_COVERAGE = $(GOTEST_OPT) -coverprofile=coverage.txt -covermode=atomic
GOTEST=go test
GOFMT=gofmt
GOIMPORTS=goimports
GOLINT=golint
GOVET=go vet
GOOS=$(shell go env GOOS)
ADDLICENCESE= addlicense
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

OTEL_DOCKER_PROTOBUF ?= otel/build-protobuf:latest
OTLP_PROTOC := docker run --rm -u ${shell id -u} -v${PWD}:${PWD} -w${PWD}/$(OTLP_PROTO_INTERMEDIATE_DIR) ${OTEL_DOCKER_PROTOBUF} --proto_path=${PWD}/$(OTLP_PROTO_INTERMEDIATE_DIR)
PROTO_INCLUDES := -I/usr/include/github.com/gogo/protobuf

# Function to execute a command. Note the empty line before endef to make sure each command
# gets executed separately instead of concatenated with previous one.
# Accepts command to execute as first parameter.
define exec-command
$(1)

endef

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
	$(foreach file,$(OTLP_PROTO_FILES),$(call exec-command, $(OTLP_PROTOC) $(PROTO_INCLUDES) --gogofaster_out=:./ $(file)))

	@echo Move generated code to target directory.
	mkdir -p $(OTLP_GEN_GO_DIR)
	cp -R $(OTLP_PROTO_INTERMEDIATE_DIR)/$(OTLP_PROTO_PACKAGE)/* $(OTLP_GEN_GO_DIR)/
	rm -rf $(OTLP_PROTO_INTERMEDIATE_DIR)

.PHONY: check
check: addlicense fmt vet lint goimports misspell staticcheck

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
	@LINTOUT=`$(GOLINT) $(ALL_PKGS) 2>&1`; \
	if [ "$$LINTOUT" ]; then \
		echo "$(GOLINT) FAILED => clean the following lint errors:\n"; \
		echo "$$LINTOUT\n"; \
		exit 1; \
	else \
	    echo "Lint finished successfully"; \
	fi

.PHONY: goimports
goimports:
	@IMPORTSOUT=`$(GOIMPORTS) -local github.com/signalfx/sapm-proto -d $(ALL_SRC) 2>&1`; \
	if [ "$$IMPORTSOUT" ]; then \
		echo "$(GOIMPORTS) FAILED => fix the following goimports errors:\n"; \
		echo "$$IMPORTSOUT\n"; \
		exit 1; \
	else \
	    echo "Goimports finished successfully"; \
	fi

.PHONY: misspell
misspell:
	$(MISSPELL) $(ALL_SRC_AND_DOC)

.PHONY: misspell-correction
misspell-correction:
	$(MISSPELL_CORRECTION) $(ALL_SRC_AND_DOC)

.PHONY: staticcheck
staticcheck:
	$(STATICCHECK) ./...

.PHONY: vet
vet:
	@$(GOVET) ./...
	@echo "Vet finished successfully"

.PHONY: fmt
fmt:
	@FMTOUT=`$(GOFMT) -s -l $(ALL_SRC) 2>&1`; \
	if [ "$$FMTOUT" ]; then \
		echo "$(GOFMT) FAILED => gofmt the following files:\n"; \
		echo "$$FMTOUT\n"; \
		exit 1; \
	else \
	    echo "Fmt finished successfully"; \
	fi

.PHONY: impi
impi:
	@$(IMPI) --local github.com/signalfx/sapm-proto --scheme stdThirdPartyLocal $(ALL_SRC)

.PHONY: install-tools
install-tools:
	GO111MODULE=on go install \
	  github.com/google/addlicense \
	  golang.org/x/lint/golint \
	  golang.org/x/tools/cmd/goimports \
	  github.com/client9/misspell/cmd/misspell \
	  honnef.co/go/tools/cmd/staticcheck \
	  github.com/pavius/impi/cmd/impi

