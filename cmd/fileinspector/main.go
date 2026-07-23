package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	ErrEmptyFile      = errors.New("file is empty")
	ErrNotRegularFile = errors.New("path is not a regular file")
	ErrFileTooLarge   = errors.New("file is too large")
)

type FileStats struct {
	Path          string
	Bytes         int64
	Lines         int
	NonEmptyLines int
	Words         int
	CommentLines  int
	LongestLine   int
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			fmt.Fprintln(os.Stderr, "File not found.")

		case errors.Is(err, os.ErrPermission):
			fmt.Fprintln(os.Stderr, "Permission denied.")

		case errors.Is(err, ErrEmptyFile):
			fmt.Fprintln(os.Stderr, "The file exists, but it is empty.")

		case errors.Is(err, ErrNotRegularFile):
			fmt.Fprintln(os.Stderr, "The supplied path is not a regular file.")

		case errors.Is(err, ErrFileTooLarge):
			fmt.Fprintln(os.Stderr, "The file is too large to inspect.")

		default:
			fmt.Fprintln(os.Stderr, "Unexpected error.")
		}

		fmt.Fprintln(os.Stderr, "Details:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: fileinspector <path>")
	}

	stats, err := inspectFile(args[0])
	if err != nil {
		return err
	}

	printStats(stats)
	return nil
}

func inspectFile(path string) (FileStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return FileStats{}, fmt.Errorf(
			"open file %q: %w",
			path,
			err,
		)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return FileStats{}, fmt.Errorf(
			"read metadata for %q: %w",
			path,
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return FileStats{}, fmt.Errorf(
			"inspect %q: %w",
			path,
			ErrNotRegularFile,
		)
	}

	const maxSize = 1 << 20 // 1 MiB

	if info.Size() > maxSize {
		return FileStats{}, fmt.Errorf(
			"inspect %q: %w",
			path,
			ErrFileTooLarge,
		)
	}

	stats := FileStats{
		Path:  path,
		Bytes: info.Size(),
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		stats.Lines++
		stats.Words += len(strings.Fields(line))

		if trimmed != "" {
			stats.NonEmptyLines++
		}

		if strings.HasPrefix(trimmed, "#") {
			stats.CommentLines++
		}

		if len(line) > stats.LongestLine {
			stats.LongestLine = len(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return FileStats{}, fmt.Errorf(
			"read contents of %q: %w",
			path,
			err,
		)
	}

	if stats.Bytes == 0 {
		return stats, fmt.Errorf(
			"inspect %q: %w",
			path,
			ErrEmptyFile,
		)
	}

	return stats, nil
}

func printStats(stats FileStats) {
	fmt.Println("File:", stats.Path)
	fmt.Println("Bytes:", stats.Bytes)
	fmt.Println("Lines:", stats.Lines)
	fmt.Println("Non-empty lines:", stats.NonEmptyLines)
	fmt.Println("Words:", stats.Words)
	fmt.Println("Comment lines:", stats.CommentLines)
	fmt.Println("Longest line:", stats.LongestLine)
}
