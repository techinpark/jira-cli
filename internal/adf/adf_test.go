package adf

import "testing"

func TestPlainTextDocAndExtract(t *testing.T) {
	doc := PlainTextDoc("hello\nworld")
	text := ExtractPlainText(doc)
	if text != "hello\n\nworld\n" {
		t.Fatalf("unexpected text: %q", text)
	}
}
