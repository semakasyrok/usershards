package users

import _ "embed"

//go:embed init.sql
var Migration string
