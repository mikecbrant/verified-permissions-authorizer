package assets

import _ "embed"

// AuthorizerIndexMjs contains the bundled Lambda authorizer entrypoint (built JS).
// This file is populated by the release workflow before building providers.
//go:embed lambda/index.mjs
var AuthorizerIndexMjs string

func GetAuthorizerIndexMjs() string { return AuthorizerIndexMjs }
