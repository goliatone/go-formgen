package loader

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"time"

	pkgjsonschema "github.com/goliatone/go-formgen/pkg/jsonschema"
)

// Loader implements pkgjsonschema.Loader by delegating to file, fs.FS, or HTTP
// strategies.
type Loader struct {
	fs        fs.FS
	http      *http.Client
	allowHTTP bool
	timeout   time.Duration
}

// Ensure the implementation satisfies the public interface.
var _ pkgjsonschema.Loader = (*Loader)(nil)

// New constructs a Loader from pre-resolved options.
func New(options pkgjsonschema.LoaderOptions) pkgjsonschema.Loader {
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
func (l *Loader) Load(ctx context.Context, src pkgjsonschema.Source) (pkgjsonschema.Document, error) {
	if src == nil {
		return pkgjsonschema.Document{}, errors.New("jsonschema loader: source is nil")
	}

	var (
		data []byte
		err  error
	)

	switch src.Kind() {
	case pkgjsonschema.SourceKindFile:
		data, err = loadFile(ctx, src.Location())
	case pkgjsonschema.SourceKindFS:
		data, err = loadFromFS(ctx, l.fs, src.Location())
	case pkgjsonschema.SourceKindURL:
		if !l.allowHTTP {
			return pkgjsonschema.Document{}, errors.New("jsonschema loader: http support disabled")
		}
		data, err = loadHTTP(ctx, l.http, src.Location(), l.timeout)
	default:
		err = errors.New("jsonschema loader: unsupported source kind")
	}
	if err != nil {
		return pkgjsonschema.Document{}, err
	}

	return pkgjsonschema.NewDocument(src, data)
}
