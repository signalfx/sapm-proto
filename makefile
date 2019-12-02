PROTO_PACKAGE_PATH?=./gen/

.PHONY: all
all:
	$(MAKE) generate
	$(MAKE) test

.PHONY: generate
generate:
	mkdir -p gen
	docker run --rm -v $(PWD):$(PWD) -w $(PWD) znly/protoc --go_out=./gen/ -I./ -I./vendor/github.com/gogo/protobuf/ -I./vendor/ sapm.proto

.PHONY: test
test:
	go test ./...
