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
all: addlicense fmt impi vet lint goimports misspell staticcheck
	$(MAKE) generate
	$(MAKE) test

.PHONY: generate
generate:
	mkdir -p gen
	docker run --rm -v $(PWD):$(PWD) -w $(PWD) znly/protoc --gofast_out=./gen/ -I./ -I./vendor/github.com/gogo/protobuf/ -I./vendor/ sapm.proto

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
 	  github.com/golangci/golangci-lint/cmd/golangci-lint \
	  golang.org/x/tools/cmd/goimports \
	  github.com/client9/misspell/cmd/misspell \
	  honnef.co/go/tools/cmd/staticcheck \
	  github.com/pavius/impi/cmd/impi

