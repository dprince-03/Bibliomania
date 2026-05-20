package utils

import (
	"fmt"
	"path/filepath"
	"strings"
)

var allowedFormats = map[string]string{
	".pdf":  "pdf",
	".epub": "epub",
	".mobi": "mobi",
	".txt":  "txt",
	".docx": "docx",
}

func ValidateFileFormat(filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	format, ok := allowedFormats[ext]
	if !ok {
		return "", fmt.Errorf("Unsupported file format: %s (allowed: pdf, epub, mobi, txt, docx)", ext)
	}

	return format, nil
}

func ValidateFileSize(sizeBytes, maxSizeMB int64) error {
	maxBytes := maxSizeMB * 1024 * 1024
	if sizeBytes > maxBytes {
		return fmt.Errorf("File size %.2fMB exceeds maximum allowed size of %dMB", float64(sizeBytes)/1024/1024, maxSizeMB)
	}

	return nil
}

func SanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "",
		"\\", "",
		"..", "",
	)

	return replacer.Replace(name)
}
