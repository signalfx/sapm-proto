package sapmprotocol

const (
	// TraceEndpointV2 is the endpoint used for SAPM v2 traces.  The SAPM protocol started with v2.  There is no v1.
	TraceEndpointV2 = "/v2/trace"
	// ContentTypeHeaderName is the http header name used for Content-Type
	ContentTypeHeaderName = "Content-Type"
	// ContentTypeHeaderValue is the value used for protobuf Content-Type http headers
	ContentTypeHeaderValue = "application/x-protobuf"

	// AcceptEncodingHeaderName is the http header name used for Accept-Encoding
	AcceptEncodingHeaderName = "Accept-Encoding"
	// ContentEncodingHeaderName is the http header name used for Content-Encoding
	ContentEncodingHeaderName = "Content-Encoding"
	// GZipEncodingHeaderValue is the value used for gzipped encoding http headers
	GZipEncodingHeaderValue = "gzip"
)
