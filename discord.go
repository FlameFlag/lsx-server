package main

import (
	_ "embed"

	"lt2_reverse/lsx_server_go/internal/discord"
)

const embeddedDiscordIconName = "upload_icon.avif"

//go:embed assets/admin/upload_icon.avif
var embeddedDiscordIcon []byte

func newDiscordNotifier(opts serveOptions) *discord.Notifier {
	return discord.New(discord.Config{
		WebhookURL:       opts.discordWebhook,
		Events:           opts.discordEvents,
		IconPath:         opts.discordIcon,
		Timeout:          opts.discordTimeout,
		EmbeddedIconName: embeddedDiscordIconName,
		EmbeddedIcon:     embeddedDiscordIcon,
	})
}
