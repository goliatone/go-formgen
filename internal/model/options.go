package model

// Options configures the behaviour of the Builder. Options are constructed by
// the public adapter in pkg/model and passed into New.
type Options struct {
	Labeler func(string) string
}

func defaultOptions() Options {
	return Options{
		Labeler: DefaultLabeler,
	}
}
