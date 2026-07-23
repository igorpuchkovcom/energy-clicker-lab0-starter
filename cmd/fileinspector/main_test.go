package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")

	content := "first line\n\nsecond line\n"

	if err := os.WriteFile(
		path,
		[]byte(content),
		0o600,
	); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	stats, err := inspectFile(path)
	if err != nil {
		t.Fatalf("inspectFile() error = %v", err)
	}

	if stats.Lines != 3 {
		t.Fatalf(
			"stats.Lines = %d, want 3",
			stats.Lines,
		)
	}

	if stats.NonEmptyLines != 2 {
		t.Fatalf(
			"stats.NonEmptyLines = %d, want 2",
			stats.NonEmptyLines,
		)
	}

	if stats.Words != 4 {
		t.Fatalf(
			"stats.Words = %d, want 4",
			stats.Words,
		)
	}
}

func TestInspectFileNotFound(t *testing.T) {
	t.Parallel()

	path := filepath.Join(
		t.TempDir(),
		"missing.txt",
	)

	_, err := inspectFile(path)

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(
			"error = %v, want os.ErrNotExist",
			err,
		)
	}
}

func TestInspectEmptyFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(
		t.TempDir(),
		"empty.txt",
	)

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	_, err := inspectFile(path)

	if !errors.Is(err, ErrEmptyFile) {
		t.Fatalf(
			"error = %v, want ErrEmptyFile",
			err,
		)
	}
}

func TestInspectDirectory(t *testing.T) {
	t.Parallel()

	_, err := inspectFile(t.TempDir())

	if !errors.Is(err, ErrNotRegularFile) {
		t.Fatalf(
			"error = %v, want ErrNotRegularFile",
			err,
		)
	}
}
