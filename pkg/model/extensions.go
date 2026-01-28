package model

import internalmodel "github.com/goliatone/go-formgen/internal/model"

// ParseUIExtensions extracts metadata and UI hints from x-formgen/x-admin
// extensions. It returns nil maps when no supported metadata is found.
func ParseUIExtensions(ext map[string]any) (map[string]string, map[string]string) {
	return internalmodel.ParseUIExtensions(ext)
}
