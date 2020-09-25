# sapm-proto

[![Circle CI](https://circleci.com/gh/signalfx/sapm-proto.svg?style=svg)](https://circleci.com/gh/signalfx/sapm-proto)
[![Docs](https://godoc.org/github.com/signalfx/sapm-proto?status.svg)](https://pkg.go.dev/github.com/signalfx/sapm-proto)
[![Go Report Card](https://goreportcard.com/badge/github.com/signalfx/sapm-proto)](https://goreportcard.com/report/github.com/signalfx/sapm-proto)

SAPM (Splunk APM Protocol) ProtoBuf schema.

Schema definition is in samp.proto and imports Jaeger model.proto that is vendored.

Use `make` to generate Go ProtoBuf code.