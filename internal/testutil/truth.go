// Package testutil provides helpers for tests, including the approved-truth
// (golden-file) pattern.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VerifyApprovedString compares got against an approved truth file.
//
// The approved file is stored at:
//
//	testdata/<TestName>.approved.txt
//
// If a suffix is provided, the file is:
//
//	testdata/<TestName>.<suffix>.approved.txt
//
// When the file does not exist, or the environment variable UPDATE_TRUTH is
// set to "1", the file is written with the current value of got.
// On mismatch, the test fails with a diff-style message.
func VerifyApprovedString(t testing.TB, got string, suffix ...string) {
	t.Helper()
	VerifyApproved(t, []byte(got), suffix...)
}

// VerifyApproved compares got against an approved truth file.
// See VerifyApprovedString for file naming and update rules.
func VerifyApproved(t testing.TB, got []byte, suffix ...string) {
	t.Helper()

	name := approvedFileName(t, suffix)
	path := filepath.Join("testdata", name)

	if os.Getenv("UPDATE_TRUTH") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write approved truth %q: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("create testdata dir: %v", err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatalf("write approved truth %q: %v", path, err)
			}
			t.Logf("approved truth created: %q", path)
			return
		}
		t.Fatalf("read approved truth %q: %v", path, err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("approved truth mismatch: %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// VerifyApprovedJSON marshals got as indented JSON and compares it against an
// approved truth file. See VerifyApprovedString for file naming and update rules.
func VerifyApprovedJSON(t testing.TB, got any, suffix ...string) {
	t.Helper()
	b, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	VerifyApproved(t, b, suffix...)
}

func approvedFileName(t testing.TB, suffix []string) string {
	name := strings.ReplaceAll(t.Name(), "/", "_")
	if len(suffix) > 0 && suffix[0] != "" {
		name = fmt.Sprintf("%s.%s.approved.txt", name, suffix[0])
	} else {
		name = name + ".approved.txt"
	}
	return name
}
