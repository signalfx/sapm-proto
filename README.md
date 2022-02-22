# sapm-proto

[![Go](https://github.com/signalfx/sapm-proto/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/signalfx/sapm-proto/actions/workflows/go.yml)
[![Docs](https://godoc.org/github.com/signalfx/sapm-proto?status.svg)](https://pkg.go.dev/github.com/signalfx/sapm-proto)
[![Go Report Card](https://goreportcard.com/badge/github.com/signalfx/sapm-proto)](https://goreportcard.com/report/github.com/signalfx/sapm-proto)

SAPM (Splunk APM Protocol) ProtoBuf schema.

Schema definition is in samp.proto and imports Jaeger model.proto that is vendored.

Use `make` to generate Go ProtoBuf code.

>ℹ️&nbsp;&nbsp;SignalFx was acquired by Splunk in October 2019. See [Splunk SignalFx](https://www.splunk.com/en_us/investor-relations/acquisitions/signalfx.html) for more information.
