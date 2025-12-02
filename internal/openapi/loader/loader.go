package loader

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"time"

	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
)

// Loader implements pkgopenapi.Loader by delegating to file, fs.FS, or HTTP
// strategies. Construction helpers live in the top-level formgen package.
type Loader struct {
	fs        fs.FS
	http      *http.Client
	allowHTTP bool
	timeout   time.Duration
}

// Ensure the implementation satisfies the public interface.
var _ pkgopenapi.Loader = (*Loader)(nil)

// New constructs a Loader from pre-resolved options.
func New(options pkgopenapi.LoaderOptions) pkgopenapi.Loader {
	timeout := options.RequestTimeout

	var httpClient *http.Client
	switch {
	case options.HTTPClient != nil:
		clone := *options.HTTPClient
		if timeout > 0 && clone.Timeout == 0 {
			clone.Timeout = timeout
		}
		httpClient = &clone
	case options.AllowHTTPFallback:
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Loader{
		fs:        options.FileSystem,
		http:      httpClient,
		allowHTTP: httpClient != nil,
		timeout:   timeout,
	}
}

// Load fetches a document from the provided source and wraps it in a Document.
func (l *Loader) Load(ctx context.Context, src pkgopenapi.Source) (pkgopenapi.Document, error) {
	if src == nil {
		return pkgopenapi.Document{}, errors.New("openapi loader: source is nil")
	}

	var (
		data []byte
		err  error
	)

	switch src.Kind() {
	case pkgopenapi.SourceKindFile:
		data, err = loadFile(ctx, src.Location())
	case pkgopenapi.SourceKindFS:
		data, err = loadFromFS(ctx, l.fs, src.Location())
	case pkgopenapi.SourceKindURL:
		if !l.allowHTTP {
			return pkgopenapi.Document{}, errors.New("openapi loader: http support disabled")
		}
		data, err = loadHTTP(ctx, l.http, src.Location(), l.timeout)
	default:
		err = errors.New("openapi loader: unsupported source kind")
	}
	if err != nil {
		return pkgopenapi.Document{}, err
	}

	return pkgopenapi.NewDocument(src, data)
}
