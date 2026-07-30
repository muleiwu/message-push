package constants

const (
	WebhookEventSuccess   = "success"
	WebhookEventFailed    = "failed"
	WebhookEventDelivered = "delivered"
	WebhookEventRejected  = "rejected"
	WebhookEventUpstream  = "upstream"
)

const (
	WebhookDeliveryPending    = "pending"
	WebhookDeliveryProcessing = "processing"
	WebhookDeliverySuccess    = "success"
	WebhookDeliveryFailed     = "failed"
)

const (
	DefaultWebhookEvents  = "success,failed,delivered,rejected,upstream"
	DefaultWebhookRetries = 3
	DefaultWebhookTimeout = 5
)
