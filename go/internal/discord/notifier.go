package discord

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"lt2_reverse/lsx_server_go/internal/lsx"
)

const defaultTimeout = 5 * time.Second

type Config struct {
	WebhookURL       string
	Events           string
	IconPath         string
	Timeout          time.Duration
	Client           *http.Client
	EmbeddedIconName string
	EmbeddedIcon     []byte
}

type Notifier struct {
	webhookURL       string
	kinds            map[string]bool
	iconPath         string
	timeout          time.Duration
	client           *http.Client
	embeddedIconName string
	embeddedIcon     []byte
}

func New(cfg Config) *Notifier {
	if cfg.WebhookURL == "" {
		return nil
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &Notifier{
		webhookURL:       cfg.WebhookURL,
		kinds:            parseKindSet(cfg.Events),
		iconPath:         cfg.IconPath,
		timeout:          timeout,
		client:           client,
		embeddedIconName: cfg.EmbeddedIconName,
		embeddedIcon:     cfg.EmbeddedIcon,
	}
}

func (n *Notifier) Sink() lsx.EventSink {
	if n == nil {
		return nil
	}
	return func(ev lsx.Event) {
		if !n.kinds[ev.Kind] {
			return
		}
		go n.send(ev)
	}
}

func (n *Notifier) send(ev lsx.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()

	payload, attachment, err := n.payload(ev)
	if err != nil {
		log.Error("discord webhook payload", "err", err)
		return
	}

	body, contentType, err := encodePayload(payload, attachment)
	if err != nil {
		log.Error("discord webhook encode", "err", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, WebhookEndpoint(n.webhookURL), body)
	if err != nil {
		log.Error("discord webhook request", "err", err)
		return
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := n.client.Do(req)
	if err != nil {
		log.Error("discord webhook post", "err", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn("discord webhook status", "status", resp.Status)
	}
}

func parseKindSet(value string) map[string]bool {
	kinds := make(map[string]bool)
	for part := range strings.SplitSeq(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			kinds[part] = true
		}
	}
	return kinds
}
