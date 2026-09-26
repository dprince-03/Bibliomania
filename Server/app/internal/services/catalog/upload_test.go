package catalog

import "testing"

func TestSniffFormat(t *testing.T) {
	epub := []byte("PK\x03\x04" + string(make([]byte, 26)) + "mimetypeapplication/epub+zip")
	cases := map[string][]byte{
		"pdf":  []byte("%PDF-1.7\n..."),
		"epub": epub,
		"":     []byte("<html><script>alert(1)</script>"),
	}
	cases["zip-not-epub"] = []byte("PK\x03\x04" + string(make([]byte, 26)) + "word/document.xml...........")
	for want, head := range cases {
		if want == "zip-not-epub" {
			want = ""
		}
		if got := sniffFormat(head); got != want {
			t.Errorf("sniff(%q...) = %q, want %q", head[:8], got, want)
		}
	}
}

func TestDownloadName(t *testing.T) {
	cases := map[string]string{
		"Harry Potter & the Philosopher's Stone": "harry-potter-the-philosopher-s-stone.pdf",
		`"; rm -rf / #`:                          "rm-rf.pdf",
		"":                                       "book.pdf",
		"日本語":                                    "book.pdf",
	}
	for title, want := range cases {
		if got := downloadName(title, "pdf"); got != want {
			t.Errorf("downloadName(%q) = %q, want %q", title, got, want)
		}
	}
	long := downloadName(string(make([]rune, 0))+"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "epub")
	if len(long) > 85 {
		t.Errorf("name not capped: %d chars", len(long))
	}
}
