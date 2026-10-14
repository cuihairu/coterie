package notification

import (
	"strings"
	"testing"
)

func TestRenderMessage(t *testing.T) {
	msg := string(renderMessage("a@x", "b@y", "Hello", "World"))
	for _, want := range []string{"From: a@x\r\n", "To: b@y\r\n", "Subject: Hello\r\n", "text/plain", "\r\n\r\nWorld"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %q", want, msg)
		}
	}
}

func TestRenderMessageStripsCRLF(t *testing.T) {
	msg := string(renderMessage("a@x", "b@y", "Hello\r\nBcc: evil@x", "World"))
	if strings.Contains(msg, "\r\nBcc:") {
		t.Fatalf("CRLF injection in subject: %q", msg)
	}
	if !strings.Contains(msg, "Subject: HelloBcc: evil@x\r\n") {
		t.Fatalf("subject not stripped: %q", msg)
	}
}
