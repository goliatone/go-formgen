package loader

import (
	"context"
	"errors"
	"io/fs"
)

func loadFromFS(ctx context.Context, files fs.FS, name string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("jsonschema loader: fs path is required")
	}
	if files == nil {
		return nil, errors.New("jsonschema loader: fs is nil")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := fs.ReadFile(files, name)
	if err != nil {
		return nil, err
	}
	return data, nil
}
