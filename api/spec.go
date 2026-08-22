package api

import _ "embed"

// OpenAPISpec is the swag-generated API document (make openapi → api/swagger.yaml).
// Served at GET /api/openapi.yaml for a stable client URL.
//
//go:embed swagger.yaml
var OpenAPISpec []byte
