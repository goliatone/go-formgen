package loader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return data, nil
}
