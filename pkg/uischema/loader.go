package uischema

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFS walks the provided filesystem and parses JSON/YAML UI schema files.
// When fsys is nil or no schema files are present, the returned store is empty.
func LoadFS(fsys fs.FS) (*Store, error) {
	store := &Store{operations: make(map[string]Operation)}
	if fsys == nil {
		return store, nil
	}

	err := fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !isSchemaFile(path) {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("uischema: read %s: %w", path, err)
		}

		doc, err := parseDocument(data, path)
		if err != nil {
			return err
		}

		presets, err := normalisePresets(doc.FieldOrderPresets, path)
		if err != nil {
			return err
		}

		for opID, raw := range doc.Operations {
			id := strings.TrimSpace(opID)
			if id == "" {
				return fmt.Errorf("uischema: file %s defines an empty operation id", path)
			}
			if _, exists := store.operations[id]; exists {
				return fmt.Errorf("uischema: duplicate operation %q (file %s)", id, path)
			}

			op, err := normaliseOperation(raw, id, path, presets)
			if err != nil {
				return err
			}
			store.operations[id] = op
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return store, nil
}

// Operation returns the configuration for the supplied operation id.
func (s *Store) Operation(id string) (Operation, bool) {
	if s == nil {
		return Operation{}, false
	}
	op, ok := s.operations[id]
	return op, ok
}

// Empty reports whether the store holds any operations.
func (s *Store) Empty() bool {
	return s == nil || len(s.operations) == 0
}

type documentFile struct {
	FieldOrderPresets map[string][]string      `json:"fieldOrderPresets" yaml:"fieldOrderPresets"`
	Operations        map[string]operationFile `json:"operations" yaml:"operations"`
}

type operationFile struct {
	Form     FormConfig             `json:"form" yaml:"form"`
	Sections []SectionConfig        `json:"sections" yaml:"sections"`
	Fields   map[string]FieldConfig `json:"fields" yaml:"fields"`
}

func parseDocument(data []byte, source string) (documentFile, error) {
	var doc documentFile
	if len(strings.TrimSpace(string(data))) == 0 {
		return documentFile{}, fmt.Errorf("uischema: file %s is empty", source)
	}

	if err := json.Unmarshal(data, &doc); err == nil {
		return doc, nil
	}

	if err := yaml.Unmarshal(data, &doc); err == nil {
		return doc, nil
	}

	return documentFile{}, fmt.Errorf("uischema: parse %s: invalid JSON or YAML", source)
}

func normaliseOperation(raw operationFile, id, source string, presets map[string][]string) (Operation, error) {
	op := Operation{
		ID:                id,
		Source:            source,
		Form:              raw.Form,
		Sections:          append([]SectionConfig(nil), raw.Sections...),
		Fields:            make(map[string]FieldConfig, len(raw.Fields)),
		FieldOrderPresets: clonePresetMap(presets),
	}

	for key, cfg := range raw.Fields {
		normalised := NormalizeFieldPath(key)
		if normalised == "" {
			return Operation{}, fmt.Errorf("uischema: operation %q (file %s) field key %q normalises to empty path", id, source, key)
		}
		if _, exists := op.Fields[normalised]; exists {
			return Operation{}, fmt.Errorf("uischema: operation %q (file %s) defines duplicate field path %q", id, source, normalised)
		}
		cloned := cloneFieldConfig(cfg)
		cloned.OriginalPath = key
		op.Fields[normalised] = cloned
	}

	return op, nil
}

func cloneFieldConfig(cfg FieldConfig) FieldConfig {
	out := cfg
	if len(cfg.UIHints) > 0 {
		out.UIHints = make(map[string]string, len(cfg.UIHints))
		for k, v := range cfg.UIHints {
			out.UIHints[k] = v
		}
	}
	if len(cfg.Metadata) > 0 {
		out.Metadata = make(map[string]string, len(cfg.Metadata))
		for k, v := range cfg.Metadata {
			out.Metadata[k] = v
		}
	}
	if len(cfg.ComponentOptions) > 0 {
		out.ComponentOptions = make(map[string]any, len(cfg.ComponentOptions))
		for k, v := range cfg.ComponentOptions {
			out.ComponentOptions[k] = v
		}
	}
	return out
}

func isSchemaFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func normalisePresets(raw map[string][]string, source string) (map[string][]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string][]string, len(raw))
	for name, pattern := range raw {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			return nil, fmt.Errorf("uischema: file %s defines a fieldOrderPresets entry with an empty name", source)
		}
		if len(pattern) == 0 {
			return nil, fmt.Errorf("uischema: file %s preset %q is empty", source, trimmedName)
		}
		cloned := make([]string, len(pattern))
		for idx, entry := range pattern {
			value := strings.TrimSpace(entry)
			if value == "" {
				return nil, fmt.Errorf("uischema: file %s preset %q contains an empty entry at index %d", source, trimmedName, idx)
			}
			cloned[idx] = value
		}
		out[trimmedName] = cloned
	}
	return out, nil
}

func clonePresetMap(src map[string][]string) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string][]string, len(src))
	for key, values := range src {
		cloned := make([]string, len(values))
		copy(cloned, values)
		out[key] = cloned
	}
	return out
}
