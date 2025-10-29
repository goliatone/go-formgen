package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/goliatone/formgen"
	"github.com/goliatone/formgen/pkg/openapi"
)

func main() {
	ctx := context.Background()
	fixture := exampleFixture("petstore.json")

	html, err := formgen.GenerateHTML(
		ctx,
		openapi.SourceFromFile(fixture),
		"createPet",
		"",
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(html))
}

func exampleFixture(name string) string {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("example: unable to resolve path")
	}
	return filepath.Join(filepath.Dir(here), "..", "fixtures", name)
}
