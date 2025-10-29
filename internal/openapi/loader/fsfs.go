package loader

import (
	"context"
	"errors"
	"io/fs"
)

func loadFromFS(ctx context.Context, filesystem fs.FS, name string) ([]byte, error) {
	if filesystem == nil {
		return nil, errors.New("openapi loader: filesystem is not configured")
	}
	if name == "" {
		return nil, errors.New("openapi loader: fs path is required")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := fs.ReadFile(filesystem, name)
	if err != nil {
		return nil, err
	}
	return data, nil
}
