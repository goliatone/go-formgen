package timezones

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

//go:embed data/iana_timezones.txt
var dataFS embed.FS

const defaultListPath = "data/iana_timezones.txt"

var (
	defaultOnce  sync.Once
	defaultZones []string
	defaultErr   error
)

func DefaultZones() ([]string, error) {
	defaultOnce.Do(func() {
		f, err := dataFS.Open(defaultListPath)
		if err != nil {
			defaultErr = err
			return
		}
		defer func() { _ = f.Close() }()

		zones, err := LoadZones(f)
		if err != nil {
			defaultErr = err
			return
		}
		defaultZones = zones
	})

	if defaultErr != nil {
		return nil, defaultErr
	}
	return append([]string{}, defaultZones...), nil
}

func LoadZones(r io.Reader) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("timezones: missing reader")
	}

	scanner := bufio.NewScanner(r)
	zones := make([]string, 0, 512)
	seen := map[string]struct{}{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		zones = append(zones, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Strings(zones)
	return zones, nil
}
