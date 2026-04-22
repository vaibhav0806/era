package telegram

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Update is our own domain type, deliberately insulating handlers from the
// tgbotapi library so we can swap libraries or fake it in tests without
// touching handler code.
type Update struct {
	UserID int64
	ChatID int64
	Text   string
}

type Client interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
	Updates(ctx context.Context) (<-chan Update, error)
}

type realClient struct {
	api           *tgbotapi.BotAPI
	allowedUserID int64
}

// NewClient connects to the Telegram Bot API using the given token, and
// returns a Client that silently drops messages from any user whose ID
// does not match allowedUserID.
func NewClient(token string, allowedUserID int64) (Client, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram bot api: %w", err)
	}
	return &realClient{api: api, allowedUserID: allowedUserID}, nil
}

func (c *realClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	// Plain text is safer than MarkdownV2 for arbitrary content; MarkdownV2
	// requires escaping many characters and we'd rather not fight it for M0.
	if _, err := c.api.Send(msg); err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	return nil
}

func (c *realClient) Updates(ctx context.Context) (<-chan Update, error) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	ch := c.api.GetUpdatesChan(u)

	out := make(chan Update)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				c.api.StopReceivingUpdates()
				return
			case up, ok := <-ch:
				if !ok {
					return
				}
				if up.Message == nil || up.Message.From == nil {
					continue
				}
				if up.Message.From.ID != c.allowedUserID {
					// Silently drop messages from any other user.
					continue
				}
				out <- Update{
					UserID: up.Message.From.ID,
					ChatID: up.Message.Chat.ID,
					Text:   up.Message.Text,
				}
			}
		}
	}()
	return out, nil
}
