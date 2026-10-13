// Package api holds the OpenAPI description of the service.
package api

import _ "embed"

// OpenAPISpec is the OpenAPI 3 specification of the HTTP API.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
