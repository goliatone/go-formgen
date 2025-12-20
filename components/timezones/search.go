package timezones

import (
	"sort"
	"strings"
)

func Search(zones []string, query string, limit int, opts Options) []string {
	limit = clampLimit(limit, opts)
	if limit == 0 {
		return nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		if opts.EmptySearchMode == EmptySearchTop {
			if len(zones) <= limit {
				return append([]string{}, zones...)
			}
			return append([]string{}, zones[:limit]...)
		}
		return nil
	}

	q := strings.ToLower(query)
	matches := make([]matchedZone, 0, 32)
	for _, zone := range zones {
		lowerZone := strings.ToLower(zone)
		if !strings.Contains(lowerZone, q) {
			continue
		}
		matches = append(matches, matchedZone{
			name:     zone,
			isPrefix: strings.HasPrefix(lowerZone, q),
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].isPrefix != matches[j].isPrefix {
			return matches[i].isPrefix
		}
		return matches[i].name < matches[j].name
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match.name)
	}
	return out
}

func SearchOptions(zones []string, query string, limit int, opts Options) []Option {
	results := Search(zones, query, limit, opts)
	if len(results) == 0 {
		return nil
	}

	out := make([]Option, 0, len(results))
	for _, zone := range results {
		out = append(out, Option{Value: zone, Label: zone})
	}
	return out
}

type matchedZone struct {
	name     string
	isPrefix bool
}
