package formgen

import (
	"embed"
	"io/fs"
)

//go:embed pkg/runtime/assets/*.js pkg/runtime/assets/*.map pkg/runtime/assets/frameworks/*.js pkg/runtime/assets/frameworks/*.map
var embeddedRuntimeAssets embed.FS

// RuntimeAssetsFS exposes the prebuilt browser runtime bundles (committed under
// pkg/runtime/assets) so Go applications can serve them without an npm build
// step.
//
// Typical mount:
//
//	mux.Handle("/runtime/",
//	  http.StripPrefix("/runtime/",
//	    http.FileServerFS(formgen.RuntimeAssetsFS()),
//	  ),
//	)
func RuntimeAssetsFS() fs.FS {
	sub, err := fs.Sub(embeddedRuntimeAssets, "pkg/runtime/assets")
	if err != nil {
		return embeddedRuntimeAssets
	}
	return sub
}
