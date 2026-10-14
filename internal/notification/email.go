package notification

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/user"
)

// EmailChannel delivers notifications as plain-text email over SMTP.
// Unconfigured (no host) it stays inert — callers simply do not
// register it.
type EmailChannel struct {
	Host     string // SMTP host; empty disables the channel
	Port     string
	Username string
	Password string
	From     string // envelope + From header address
	users    *user.Store
}

// NewEmailChannel builds the channel; it is ready when Host is set.
func NewEmailChannel(db *gorm.DB, host, port, username, password, from string) *EmailChannel {
	return &EmailChannel{
		Host: host, Port: port, Username: username, Password: password, From: from,
		users: user.NewStore(db),
	}
}

// Name identifies the channel in delivery logs.
func (c *EmailChannel) Name() string { return "email" }

// Enabled reports whether configuration is present.
func (c *EmailChannel) Enabled() bool { return c.Host != "" && c.From != "" }

// Deliver resolves the recipient's address and mails the notification.
func (c *EmailChannel) Deliver(ctx context.Context, n *Notification) error {
	u, err := c.users.Get(ctx, n.UserID)
	if err != nil {
		return err
	}
	if u == nil || u.Email == "" {
		return nil // no known address — nothing to deliver to
	}
	addr := c.Host + ":" + c.Port
	var auth smtp.Auth
	if c.Username != "" {
		auth = smtp.PlainAuth("", c.Username, c.Password, c.Host)
	}
	to := headerSafe(u.Email)
	from := headerSafe(c.From)
	return smtp.SendMail(addr, auth, from, []string{to},
		renderMessage(from, to, headerSafe(n.Title), headerSafe(n.Body)))
}

// renderMessage builds a minimal RFC 5322 plain-text message.
func renderMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")
	return []byte(b.String())
}

// headerSafe strips CR/LF so a crafted title cannot inject additional
// SMTP headers (CodeQL go/email-injection).
func headerSafe(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}
