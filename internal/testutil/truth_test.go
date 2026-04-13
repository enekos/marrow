package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

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
