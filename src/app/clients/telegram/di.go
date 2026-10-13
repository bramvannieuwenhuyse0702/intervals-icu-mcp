package telegram

import (
	"os"

	"go.uber.org/fx"
)

// Module provides the Telegram client and its configuration.
var Module = fx.Module("telegram", //nolint:gochecknoglobals // standard DI pattern.
	fx.Provide(func() Config {
		return Config{
			BotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
			ChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		}
	}),
	fx.Provide(NewClient),
)
