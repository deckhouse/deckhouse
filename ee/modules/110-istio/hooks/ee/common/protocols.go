/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package common

var ProtocolMap = map[string]string{
	"https":    "TLS",
	"tls":      "TLS",
	"http":     "HTTP",
	"http2":    "HTTP2",
	"grpc":     "HTTP2",
	"grpc-web": "HTTP2",
}

var DefaultProtocol = "TCP"
