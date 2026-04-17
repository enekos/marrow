package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockTB wraps a real testing.TB and intercepts Fatalf calls so we can assert
// on failure conditions without aborting the actual test.
type mockTB struct {
	testing.TB
	fatalfCalled bool
	fatalfMsg    string
}

func (m *mockTB) Fatalf(format string, args ...any) {
	m.fatalfCalled = true
	m.fatalfMsg = fmt.Sprintf(format, args...)
}

func TestVerifyApprovedString_createsMissingFile(t *testing.T) {
	// Use a unique suffix so we don't collide with an existing file.
	suffix := "create_test"
	path := filepath.Join("testdata", approvedFileName(t, []string{suffix}))
	_ = os.Remove(path)

	VerifyApprovedString(t, "hello world", suffix)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to be created: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected file content %q, got %q", "hello world", string(data))
	}
}

func TestVerifyApprovedString_passesWhenMatching(t *testing.T) {
	suffix := "match_test"
	path := filepath.Join("testdata", approvedFileName(t, []string{suffix}))
	_ = os.WriteFile(path, []byte("matching content"), 0o644)

	VerifyApprovedString(t, "matching content", suffix)
}

func TestVerifyApprovedString_mismatchFails(t *testing.T) {
	suffix := "mismatch_test"
	path := filepath.Join("testdata", approvedFileName(t, []string{suffix}))
	_ = os.WriteFile(path, []byte("expected content"), 0o644)
	t.Cleanup(func() { _ = os.Remove(path) })

	mt := &mockTB{TB: t}
	VerifyApprovedString(mt, "actual content", suffix)

	if !mt.fatalfCalled {
		t.Fatalf("expected Fatalf to be called on mismatch")
	}
	if !strings.Contains(mt.fatalfMsg, "approved truth mismatch") {
		t.Fatalf("expected mismatch message, got %q", mt.fatalfMsg)
	}
}

func TestVerifyApproved_updateTruth(t *testing.T) {
	suffix := "update_truth_test"
	path := filepath.Join("testdata", approvedFileName(t, []string{suffix}))
	_ = os.WriteFile(path, []byte("old content"), 0o644)
	t.Cleanup(func() { _ = os.Remove(path) })

	t.Setenv("UPDATE_TRUTH", "1")
	VerifyApprovedString(t, "new content", suffix)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if string(data) != "new content" {
		t.Fatalf("expected file content %q, got %q", "new content", string(data))
	}
}

func TestApprovedFileName_noSuffix(t *testing.T) {
	want := t.Name() + ".approved.txt"

	if got := approvedFileName(t, nil); got != want {
		t.Fatalf("approvedFileName(nil) = %q, want %q", got, want)
	}
	if got := approvedFileName(t, []string{}); got != want {
		t.Fatalf("approvedFileName([]string{}) = %q, want %q", got, want)
	}
}

func TestVerifyApprovedJSON(t *testing.T) {
	suffix := "json_test"
	path := filepath.Join("testdata", approvedFileName(t, []string{suffix}))
	_ = os.Remove(path)

	type item struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	VerifyApprovedJSON(t, item{Name: "Ada", Age: 42}, suffix)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected JSON file to be created: %v", err)
	}
	want := "{\n  \"name\": \"Ada\",\n  \"age\": 42\n}"
	if string(data) != want {
		t.Fatalf("expected JSON content %q, got %q", want, string(data))
	}
}

func TestVerifyApprovedJSON_mismatch(t *testing.T) {
	suffix := "json_mismatch_test"
	path := filepath.Join("testdata", approvedFileName(t, []string{suffix}))
	_ = os.WriteFile(path, []byte("{\n  \"name\": \"Bob\"\n}"), 0o644)
	t.Cleanup(func() { _ = os.Remove(path) })

	type item struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	mt := &mockTB{TB: t}
	VerifyApprovedJSON(mt, item{Name: "Ada", Age: 42}, suffix)

	if !mt.fatalfCalled {
		t.Fatalf("expected Fatalf to be called on JSON mismatch")
	}
	if !strings.Contains(mt.fatalfMsg, "approved truth mismatch") {
		t.Fatalf("expected mismatch message, got %q", mt.fatalfMsg)
	}
}
