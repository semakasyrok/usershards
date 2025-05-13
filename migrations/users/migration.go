package users

import _ "embed"

//go:embed init.sql
var Migration1 string

//go:embed idempotence.sql
var Migration2 string

//go:embed transactions.sql
var Migration3 string
