package loader

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

func loadHTTP(ctx context.Context, client *http.Client, url string, timeout time.Duration) ([]byte, error) {
	if client == nil {
		return nil, errors.New("jsonschema loader: http client is not configured")
	}
	if url == "" {
		return nil, errors.New("jsonschema loader: url is required")
	}

	reqCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("jsonschema loader: unexpected status " + resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
