package loader

import (
	"context"
	"errors"

	"github.com/goliatone/go-formgen/internal/safefile"
)

func loadFile(ctx context.Context, path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("jsonschema loader: file path is required")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := safefile.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}
