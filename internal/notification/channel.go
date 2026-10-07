package notification

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// dispatchTimeout bounds each outbound delivery attempt.
const dispatchTimeout = 10 * time.Second

// Channel is one outbound delivery path (design FR-12): email, webhook,
// later push. Channels are best-effort — a failed delivery is logged,
// never fatal, and never blocks the domain transaction that produced
// the notification.
type Channel interface {
	Name() string
	Deliver(ctx context.Context, n *Notification) error
}

// NewServiceWithChannels builds a Service that fans every notification
// out to the given channels in the background, in addition to writing
// the in-app row.
func NewServiceWithChannels(db *gorm.DB, log *slog.Logger, channels ...Channel) *Service {
	return &Service{store: NewStore(db), log: log, channels: channels}
}

// dispatchAsync delivers the notification to every channel in the
// background. Panics in a channel must not take the server down.
func (s *Service) dispatchAsync(n *Notification) {
	if len(s.channels) == 0 {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("notification channel panicked", "channel", "unknown", "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
		defer cancel()
		for _, ch := range s.channels {
			if err := ch.Deliver(ctx, n); err != nil {
				s.log.Warn("notification delivery failed",
					"channel", ch.Name(), "user_id", n.UserID, "type", n.Type, "err", err)
			}
		}
	}()
}
