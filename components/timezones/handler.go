package timezones

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type HTTPError interface {
	error
	StatusCode() int
}

type StatusError struct {
	Code int
	Err  error
}

func (e StatusError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.Code)
}

func (e StatusError) Unwrap() error { return e.Err }

func (e StatusError) StatusCode() int {
	if e.Code <= 0 {
		return http.StatusInternalServerError
	}
	return e.Code
}

type optionsResponse struct {
	Data []Option `json:"data"`
}

// Handler builds a net/http handler with default options plus any overrides.
// It is an alias of NewHandler to match the recommended component API surface.
func Handler(fns ...OptionFn) http.Handler {
	return NewHandler(fns...)
}

func NewHandler(fns ...OptionFn) http.Handler {
	opts := NewOptions(fns...)
	return HandlerWithOptions(opts)
}

// HandlerWithOptions builds a net/http handler from a pre-constructed Options value.
// Callers are expected to pass an Options value produced by NewOptions (or equivalent)
// so defaults/clamps are applied.
func HandlerWithOptions(opts Options) http.Handler {
	opts = NewOptions(func(o *Options) { *o = opts })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if opts.Guard != nil {
			if err := opts.Guard(r); err != nil {
				writeGuardError(w, err)
				return
			}
		}

		zones := opts.Zones
		if zones == nil {
			loaded, err := DefaultZones()
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			zones = loaded
		}

		query := r.URL.Query().Get(opts.SearchParam)
		limit := parseInt(r.URL.Query().Get(opts.LimitParam))

		results := SearchOptions(zones, query, limit, opts)
		if results == nil {
			results = []Option{}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}

		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(true)
		_ = enc.Encode(optionsResponse{Data: results})
	})
}

func writeGuardError(w http.ResponseWriter, err error) {
	if w == nil {
		return
	}
	if err == nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	code := http.StatusForbidden
	var httpErr HTTPError
	if errors.As(err, &httpErr) && httpErr != nil {
		code = httpErr.StatusCode()
		if code <= 0 {
			code = http.StatusForbidden
		}
	}
	http.Error(w, http.StatusText(code), code)
}

func parseInt(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}
