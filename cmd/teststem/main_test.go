package main

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(out)
}

func TestPrintStemmed(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		lang      string
		wantEmpty bool
	}{
		{
			name: "english simple",
			text: "running dogs",
			lang: "en",
		},
		{
			name: "spanish simple",
			text: "corriendo rápido",
			lang: "es",
		},
		{
			name: "basque simple",
			text: "nire etxea",
			lang: "eu",
		},
		{
			name:      "empty string",
			text:      "",
			lang:      "en",
			wantEmpty: true,
		},
		{
			name:      "whitespace only",
			text:      "   \t\n  ",
			lang:      "en",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				printStemmed(tt.text, tt.lang)
			})

			if tt.wantEmpty {
				if out != "" {
					t.Errorf("printStemmed(%q, %q) = %q, want empty", tt.text, tt.lang, out)
				}
				return
			}

			if !strings.Contains(out, "tokens:") {
				t.Errorf("output missing tokens line: %q", out)
			}
			if !strings.Contains(out, "filtered:") {
				t.Errorf("output missing filtered line: %q", out)
			}
			if !strings.Contains(out, "stemmed:") {
				t.Errorf("output missing stemmed line: %q", out)
			}
		})
	}
}

func TestMain_supportedLang(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"teststem", "-lang", "es", "corriendo"}

	// Reset flags so that flag.Parse re-parses the new os.Args.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	out := captureStdout(t, func() {
		if code := run(); code != 0 {
			t.Fatalf("run() returned %d, want 0", code)
		}
	})

	if !strings.Contains(out, "stemmed:") {
		t.Errorf("output missing stemmed line: %q", out)
	}
}

func TestMain_unsupportedLang(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"teststem", "-lang", "fr", "test"}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	if code := run(); code != 1 {
		t.Errorf("run() returned %d, want 1", code)
	}
}

func TestMain_stdin(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"teststem", "-lang", "en"}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = r

	go func() {
		w.WriteString("running dogs\n")
		w.Close()
	}()

	out := captureStdout(t, func() {
		if code := run(); code != 0 {
			t.Fatalf("run() returned %d, want 0", code)
		}
	})

	os.Stdin = oldStdin

	if !strings.Contains(out, "stemmed:") {
		t.Errorf("output missing stemmed line: %q", out)
	}
}
