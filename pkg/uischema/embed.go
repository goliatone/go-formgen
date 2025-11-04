package uischema

import (
	"embed"
	"io/fs"
)

//go:embed ui/schema/*
var embeddedSchema embed.FS

// EmbeddedFS returns the bundled UI schema assets. Callers may pass this
// filesystem to LoadFS to use the default configuration.
func EmbeddedFS() fs.FS {
	sub, err := fs.Sub(embeddedSchema, "ui/schema")
	if err != nil {
		// The embed directive guarantees the subpath exists, so panic is
		// acceptable here.
		panic(err)
	}
	return sub
}
