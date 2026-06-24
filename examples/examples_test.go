//go:build example

package examples_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const exampleCommandTimeout = 2 * time.Minute

var listeningLinePattern = regexp.MustCompile(`listening on ([^ ]+)`)

var examplePackages = []struct {
	name string
	dir  string
	pkg  string
}{
	{name: "basic", pkg: "./examples/basic"},
	{name: "cli", pkg: "./examples/cli"},
	{name: "http", dir: "examples/http", pkg: "."},
	{name: "multi-renderer", pkg: "./examples/multi-renderer"},
}

func TestExamplesCompile(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	for _, example := range examplePackages {
		example := example
		t.Run(example.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), example.name)
			cmd := exec.Command(goBin(), "build", "-o", outputPath, example.pkg)
			cmd.Dir = repoRoot
			if example.dir != "" {
				cmd.Dir = filepath.Join(repoRoot, example.dir)
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go build %s: %v\n%s", example.pkg, err, output)
			}
		})
	}
}

func TestExamplesRun(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	t.Run("basic", func(t *testing.T) {
		output := runExampleCommand(t, repoRoot, goBin(), "run", "./examples/basic")
		if !bytes.Contains(output, []byte("<form")) {
			t.Fatalf("basic example output did not contain a form")
		}
	})

	t.Run("cli", func(t *testing.T) {
		output := runExampleCommand(t, repoRoot, goBin(), "run", "./examples/cli")
		if !bytes.Contains(output, []byte("<form")) {
			t.Fatalf("cli example output did not contain a form")
		}
	})

	t.Run("multi-renderer", func(t *testing.T) {
		outputDir := t.TempDir()
		_ = runExampleCommand(t, repoRoot, goBin(), "run", "./examples/multi-renderer", "-output", outputDir)
		for _, name := range []string{"preact.html", "vanilla.html"} {
			data, err := os.ReadFile(filepath.Join(outputDir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if len(data) == 0 {
				t.Fatalf("%s was empty", name)
			}
		}
	})

	t.Run("http-quick-navigation", func(t *testing.T) {
		httpDir := filepath.Join(repoRoot, "examples/http")
		binary := filepath.Join(t.TempDir(), "http-example")
		runExampleCommand(t, httpDir, goBin(), "build", "-o", binary, ".")

		var output lockedBuffer
		cmd := exec.Command(binary, "--addr", "127.0.0.1:0")
		cmd.Stdout = &output
		stderr, err := cmd.StderrPipe()
		if err != nil {
			t.Fatalf("capture http example stderr: %v", err)
		}
		addrCh := make(chan string, 1)
		go scanHTTPExampleOutput(stderr, &output, addrCh)
		if err := cmd.Start(); err != nil {
			t.Fatalf("start http example: %v", err)
		}
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		defer func() {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(os.Interrupt)
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				<-done
			}
		}()

		addr := waitForHTTPExampleAddress(t, addrCh, &output)
		client := &http.Client{Timeout: 5 * time.Second}
		baseURL := "http://" + addr
		waitForHTTP(t, client, baseURL+"/")
		for _, target := range []string{
			"/form",
			"/advanced",
			"/form?renderer=preact",
			"/form?id=article-edit&method=PATCH",
		} {
			resp, err := client.Get(baseURL + target)
			if err != nil {
				t.Fatalf("GET %s: %v\n%s", target, err, output.String())
			}
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatalf("read %s response: %v", target, readErr)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s returned %d:\n%s\nserver output:\n%s", target, resp.StatusCode, string(body), output.String())
			}
			if strings.Contains(string(body), "already registered") {
				t.Fatalf("GET %s hit duplicate renderer registration:\n%s", target, string(body))
			}
		}
	})
}

func runExampleCommand(t *testing.T, dir string, name string, args ...string) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), exampleCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s %s timed out\n%s", name, strings.Join(args, " "), string(output))
	}
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
	return output
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func scanHTTPExampleOutput(stderr io.Reader, output io.Writer, addrCh chan<- string) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		_, _ = fmt.Fprintln(output, line)
		if match := listeningLinePattern.FindStringSubmatch(line); len(match) == 2 {
			select {
			case addrCh <- match[1]:
			default:
			}
		}
	}
	close(addrCh)
}

func waitForHTTPExampleAddress(t *testing.T, addrCh <-chan string, output fmt.Stringer) string {
	t.Helper()

	select {
	case addr, ok := <-addrCh:
		if ok && addr != "" {
			return addr
		}
		t.Fatalf("http example exited before reporting listen address\n%s", output.String())
	case <-time.After(30 * time.Second):
		t.Fatalf("http example did not report listen address\n%s", output.String())
	}
	return ""
}

func waitForHTTP(t *testing.T, client *http.Client, url string) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s: %v", url, lastErr)
}

func goBin() string {
	if goBin := os.Getenv("GO_BIN"); goBin != "" {
		return goBin
	}
	return "go"
}
