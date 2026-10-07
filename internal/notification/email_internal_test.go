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
