package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParsePagination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	limit, offset := parsePagination(req)
	if limit != 50 || offset != 0 {
		t.Fatalf("expected defaults 50/0, got %d/%d", limit, offset)
	}

	req = httptest.NewRequest(http.MethodGet, "/tasks?limit=25&offset=8", nil)
	limit, offset = parsePagination(req)
	if limit != 25 || offset != 8 {
		t.Fatalf("expected parsed values 25/8, got %d/%d", limit, offset)
	}

	req = httptest.NewRequest(http.MethodGet, "/tasks?limit=bad&offset=also-bad", nil)
	limit, offset = parsePagination(req)
	if limit != 50 || offset != 0 {
		t.Fatalf("expected invalid values to fall back to defaults, got %d/%d", limit, offset)
	}
}

func TestParseTaskListParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tasks?status=pending,accepted&priority=high,urgent&tag=legal&limit=12&offset=3&sort=created_at", nil)
	params := parseTaskListParams(req)

	if params.Sort != "created_at" {
		t.Fatalf("expected sort to be preserved, got %q", params.Sort)
	}
	if params.Tag != "legal" {
		t.Fatalf("expected tag to be preserved, got %q", params.Tag)
	}
	if got := strings.Join(params.Status, ","); got != "pending,accepted" {
		t.Fatalf("expected split status list, got %q", got)
	}
	if got := strings.Join(params.Priority, ","); got != "high,urgent" {
		t.Fatalf("expected split priority list, got %q", got)
	}
	if params.Limit != 12 || params.Offset != 3 {
		t.Fatalf("expected limit/offset 12/3, got %d/%d", params.Limit, params.Offset)
	}
}

func TestStripHTMLTags(t *testing.T) {
	got := stripHTMLTags(`<b>Hello</b> <script>alert("x")</script> world`)
	if got != `Hello alert("x") world` {
		t.Fatalf("unexpected stripped output: %q", got)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := map[string]string{
		"../../etc/passwd": "passwd",
		"weird name?.txt":  "weird_name_.txt",
		"":                 "file",
		"..":               "file",
	}

	for input, want := range tests {
		if got := sanitizeFilename(input); got != want {
			t.Fatalf("sanitizeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBytesReader(t *testing.T) {
	r := bytes_reader([]byte("hello"))
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(data))
	}
}

func TestStrPtrAndDerefStr(t *testing.T) {
	value := "taskboard"
	if got := derefStr(strPtr(value)); got != value {
		t.Fatalf("expected round-trip string pointer, got %q", got)
	}
	if got := derefStr(nil); got != "" {
		t.Fatalf("expected nil pointer to dereference to empty string, got %q", got)
	}
}
