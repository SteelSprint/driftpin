package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
)

// #F id:mzvtfvxt marker_format.content_window
const defaultContentWindow = 10

func ComputeContentHash(path string, markerLine int, windowSize int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	lines, err := readLines(f)
	if err != nil {
		return "", err
	}

	start := markerLine // 0-indexed: markerLine is 1-indexed, so start is the line after
	end := start + windowSize
	if start >= len(lines) {
		return hashOfString(""), nil
	}
	if end > len(lines) {
		end = len(lines)
	}

	content := strings.Join(lines[start:end], "\n")
	content = normalizeContent(content)
	return hashOfString(content), nil
}

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// #F id:zyph0jce marker_format.content_normalization
func normalizeContent(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	prevBlank := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		isBlank := trimmed == ""
		if isBlank && prevBlank {
			continue
		}
		out = append(out, trimmed)
		prevBlank = isBlank
	}
	return strings.Join(out, "\n")
}

func hashOfString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
