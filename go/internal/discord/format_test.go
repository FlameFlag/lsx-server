package discord

import "testing"

func TestWebhookEndpointAddsWait(t *testing.T) {
	got := WebhookEndpoint("https://discord.com/api/webhooks/id/token")
	if got != "https://discord.com/api/webhooks/id/token?wait=true" {
		t.Fatalf("WebhookEndpoint() = %q", got)
	}

	got = WebhookEndpoint("https://discord.com/api/webhooks/id/token?wait=false&thread_id=123")
	want := "https://discord.com/api/webhooks/id/token?thread_id=123&wait=false"
	if got != want {
		t.Fatalf("WebhookEndpoint() = %q, want %q", got, want)
	}
}
