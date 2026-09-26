package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/storage"
	"github.com/dprince-03/Bibliomania/pkg/clamav"

	"github.com/google/uuid"
)

// Scanner virus-scans an upload (pkg/clamav in production). nil in the
// BookService means scanning is disabled (CLAMAV_ADDR unset).
type Scanner interface {
	Scan(ctx context.Context, r io.Reader) error
}

var contentTypes = map[string]string{
	"pdf":  "application/pdf",
	"epub": "application/epub+zip",
}

// sniffFormat identifies a book file from its first bytes — the extension
// and the multipart Content-Type are both client-controlled and prove
// nothing.
//   - PDF: starts with "%PDF-".
//   - EPUB: a ZIP whose first entry is an uncompressed file named
//     "mimetype" containing "application/epub+zip" (required by the EPUB
//     spec precisely so it can be identified this way): local file header
//     signature PK\x03\x04, name at offset 30, content right after.
func sniffFormat(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return "pdf"
	case len(head) >= 58 && bytes.HasPrefix(head, []byte("PK\x03\x04")) &&
		string(head[30:38]) == "mimetype" && string(head[38:58]) == "application/epub+zip":
		return "epub"
	default:
		return ""
	}
}

// storeUpload validates, scans and stores an uploaded book file, returning
// its storage key, size and format. The steps:
//  1. spool to a temp file, counting bytes ourselves — the declared size is
//     client-supplied (the handler's MaxBytesReader is the hard cap);
//  2. sniff the real format from the content, and require it to match the
//     extension;
//  3. virus-scan the spooled file (if a scanner is configured — fail closed
//     if it is but can't be reached);
//  4. store under a generated key — never the uploader's filename, so no
//     path tricks, collisions or overwrites of another book's file.
func (s *BookService) storeUpload(ctx context.Context, bookID uint64, filename string, src io.Reader) (key string, size int64, format string, err error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if _, ok := contentTypes[ext]; !ok {
		return "", 0, "", apperrors.BadRequest("only .pdf and .epub files are supported for books", nil)
	}

	tmp, err := os.CreateTemp("", "bibliomania-upload-*")
	if err != nil {
		return "", 0, "", apperrors.Internal(err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	maxBytes := s.maxUploadMB * 1024 * 1024
	size, err = io.Copy(tmp, io.LimitReader(src, maxBytes+1))
	if err != nil {
		return "", 0, "", apperrors.BadRequest("upload interrupted", err)
	}
	if size > maxBytes {
		return "", 0, "", apperrors.BadRequest(fmt.Sprintf("file exceeds the maximum size of %d MB", s.maxUploadMB), nil)
	}
	if size == 0 {
		return "", 0, "", apperrors.BadRequest("file is empty", nil)
	}

	head := make([]byte, 64)
	n, _ := tmp.ReadAt(head, 0)
	format = sniffFormat(head[:n])
	if format == "" {
		return "", 0, "", apperrors.BadRequest("file content is not a valid PDF or EPUB", nil)
	}
	if format != ext {
		return "", 0, "", apperrors.BadRequest(fmt.Sprintf("file content is %s but the name says .%s", format, ext), nil)
	}

	if s.scanner != nil {
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return "", 0, "", apperrors.Internal(err)
		}
		if err := s.scanner.Scan(ctx, tmp); err != nil {
			var infected *clamav.ErrInfected
			if errors.As(err, &infected) {
				slog.WarnContext(ctx, "upload rejected: malware detected", "book_id", bookID, "signature", infected.Signature)
				return "", 0, "", apperrors.UnprocessableEntity("file rejected by the virus scanner")
			}
			slog.ErrorContext(ctx, "virus scan unavailable — upload refused", "error", err)
			return "", 0, "", &apperrors.AppError{Code: 503, Message: "virus scanning is unavailable, try again later", Err: err}
		}
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", 0, "", apperrors.Internal(err)
	}
	key = fmt.Sprintf("books/%d/%s.%s", bookID, uuid.Must(uuid.NewV7()), format)
	if err := s.store.Put(ctx, key, tmp, size, contentTypes[format]); err != nil {
		return "", 0, "", apperrors.Internal(err)
	}
	return key, size, format, nil
}

// downloadName builds the filename offered to the reader from the book's
// title (the stored key is a UUID): letters, digits, dashes only.
func downloadName(title, format string) string {
	var b strings.Builder
	lastDash := false
chars:
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
		if b.Len() >= 80 {
			break chars
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "book"
	}
	return name + "." + format
}

// Download is an open stored file plus what the handler needs to serve it.
type Download struct {
	Object      storage.Object
	Name        string
	ContentType string
	ModTime     time.Time
}
