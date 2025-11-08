package vanilla

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.tmpl templates/components/*.tmpl templates/components/chrome/*.tmpl
var embeddedTemplates embed.FS

//go:embed assets/*
var embeddedAssets embed.FS

const (
	StylesheetName    = "formgen-vanilla.css"
	RuntimeScriptName = "formgen-relationships.min.js"
)

// TemplatesFS exposes the embedded template bundle for consumers that want to
// use the built-in form rendering out of the box.
func TemplatesFS() fs.FS {
	return embeddedTemplates
}

// AssetsFS exposes the embedded runtime asset bundle (CSS/JS) so callers can
// serve them over HTTP or copy them into their own asset pipeline.
func AssetsFS() fs.FS {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		// Should never happen, but fall back to raw FS so assets remain usable.
		return embeddedAssets
	}
	return sub
}

func defaultStylesheet() string {
	data, err := fs.ReadFile(embeddedAssets, "assets/"+StylesheetName)
	if err != nil {
		return ""
	}
	return string(data)
}
