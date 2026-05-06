package discord

import (
	"fmt"
	"net/url"
	"strings"

	"lt2_reverse/lsx_server_go/internal/lsxvalue"
)

func valuePlain(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func valueMoney(value string) string {
	if value == "" {
		return "-"
	}
	return lsxvalue.FormatCents(lsxvalue.ParseInt(value))
}

func valueWholeNumber(value string) string {
	return lsxvalue.FormatInt(lsxvalue.ParseInt(value))
}

func safeValue(value string) string {
	return valuePlain(value)
}

func bold(value string) string {
	return "**" + value + "**"
}

func relativeTime(view eventView) string {
	if view.event.Time.IsZero() {
		return ""
	}
	return fmt.Sprintf("<t:%d:R>", view.event.Time.Unix())
}

func WebhookEndpoint(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := parsed.Query()
	if q.Get("wait") == "" {
		q.Set("wait", "true")
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
