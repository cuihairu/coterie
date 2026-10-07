// Package notification delivers in-app notifications (FR-12). Domain
// modules call Notify after their own transaction commits; a failed
// notification is logged, never fatal. Email and push arrive later as
// channel adapters.
package notification
