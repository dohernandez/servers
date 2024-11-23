package testdata

import _ "embed"

// SwgJSON contains the helloworld.swagger.json definition.
//
//go:embed helloworld.swagger.json
var SwgJSON []byte
