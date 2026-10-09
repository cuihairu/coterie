// Package notification delivers in-app notifications (FR-12). Domain
// modules call Notify after their own transaction commits; a failed
// notification is logged, never fatal. Outbound channels: email (SMTP),
// webhook (HMAC-signed POST), and Web Push (D22) — each registered only
// when its configuration is present.
package notification
