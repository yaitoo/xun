package xun

import (
	"html/template"
	"strings"
)

var builtins = template.FuncMap{
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"join":  join,
}

func join(sep string, a ...string) string {
	return strings.Join(a, sep)
}
