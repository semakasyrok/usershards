package emails

import _ "embed"

//go:embed init.sql
var Migration1 string

//go:embed idempotency.sql
var Migration2 string
