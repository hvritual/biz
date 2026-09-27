package notification

// MemorySender deduplicates by EventID and rejects payload changes for a reused
// key, so it is safe for retry/recovery contract tests. This is not evidence of
// any real SMS/email provider capability.
func (sender *MemorySender) SecurityNotificationIdempotent() bool { return true }
