package safefile

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	PrivateDirPerm  os.FileMode = 0o750
	PrivateFilePerm os.FileMode = 0o600
)

// ReadFile reads a single file through an os.Root scoped to the file's parent
// directory, preventing path traversal from escaping that directory while still
// allowing callers to pass normal relative or absolute paths.
func ReadFile(path string) ([]byte, error) {
	root, name, err := openParentRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = root.Close()
	}()
	return root.ReadFile(name)
}

// WriteFile writes a single file through an os.Root scoped to the file's parent
// directory. Missing parent directories are created with restricted permissions.
func WriteFile(path string, data []byte) error {
	if err := MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	root, name, err := openParentRoot(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = root.Close()
	}()
	return root.WriteFile(name, data, PrivateFilePerm)
}

// MkdirAll creates a directory tree using restricted permissions.
func MkdirAll(path string) error {
	if path == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, PrivateDirPerm)
}

func openParentRoot(path string) (*os.Root, string, error) {
	if path == "" {
		return nil, "", fmt.Errorf("file path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return nil, "", err
	}
	return root, filepath.Base(abs), nil
}
