package logviewer

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestFormatStreamEnd_CleanCloses(t *testing.T) {
	cases := []error{nil, io.EOF, context.Canceled}
	for _, err := range cases {
		text, isErr := formatStreamEnd(err)
		if isErr {
			t.Errorf("err=%v: expected isErr=false, got true", err)
		}
		if !strings.Contains(text, "stream ended") {
			t.Errorf("err=%v: expected text to contain 'stream ended', got %q", err, text)
		}
	}
}

func TestFormatStreamEnd_RealErrorIsHighlighted(t *testing.T) {
	boom := errors.New("connection reset by peer")
	text, isErr := formatStreamEnd(boom)
	if !isErr {
		t.Error("expected isErr=true for non-EOF error")
	}
	if !strings.Contains(text, "connection reset by peer") {
		t.Errorf("expected text to include underlying error, got %q", text)
	}
}

func TestFormatStreamEnd_WrappedEOF(t *testing.T) {
	wrapped := errors.Join(errors.New("context"), io.EOF)
	_, isErr := formatStreamEnd(wrapped)
	if isErr {
		t.Error("expected wrapped io.EOF to be treated as a clean close")
	}
}
