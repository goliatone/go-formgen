package model

// ParseUIExtensions extracts metadata and UI hints from x-formgen/x-admin
// extensions. It returns nil maps when no supported metadata is found.
func ParseUIExtensions(ext map[string]any) (map[string]string, map[string]string) {
	metadata := metadataFromExtensions(ext)
	uiHints := filterUIHints(metadata)
	uiHints = mergeUIHints(uiHints, gridHintsFromExtensions(ext))
	return metadata, uiHints
}
