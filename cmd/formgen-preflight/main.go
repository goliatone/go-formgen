// formgen-preflight checks original JSON definitions before adapter/browser rounding.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/goliatone/go-formgen/internal/safefile"
	"github.com/goliatone/go-formgen/pkg/model"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: formgen-preflight definition.json [...]")
		os.Exit(2)
	}
	failed := false
	for _, path := range os.Args[1:] {
		data, err := safefile.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
			continue
		}
		diagnostics, err := model.DiagnosePortableDefinitionJSON(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
			continue
		}
		for _, d := range diagnostics {
			record := struct {
				File string `json:"file"`
				model.DefinitionDiagnostic
			}{path, d}
			_ = json.NewEncoder(os.Stdout).Encode(record)
		}
		failed = failed || len(diagnostics) > 0
	}
	if failed {
		os.Exit(1)
	}
}
