package openapi

import _ "embed"

// YAML is the control-plane OpenAPI 3 document.
//
//go:embed openapi.yaml
var YAML []byte
