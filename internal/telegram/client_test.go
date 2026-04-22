package telegram

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// FakeClient is exported so handler_test.go (Task 9) can reuse it.
type FakeClient struct {
	mu   sync.Mutex
	Sent []struct {
		ChatID int64
		Text   string
	}
	Incoming chan Update
}

func NewFakeClient() *FakeClient { return &FakeClient{Incoming: make(chan Update, 16)} }

func (f *FakeClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Sent = append(f.Sent, struct {
		ChatID int64
		Text   string
	}{chatID, text})
	return nil
}

func (f *FakeClient) Updates(ctx context.Context) (<-chan Update, error) {
	out := make(chan Update)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case u, ok := <-f.Incoming:
				if !ok {
					return
				}
				out <- u
			}
		}
	}()
	return out, nil
}

func TestFakeClient_RoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := NewFakeClient()
	updates, err := f.Updates(ctx)
	require.NoError(t, err)

	f.Incoming <- Update{UserID: 1, ChatID: 1, Text: "hi"}
	got := <-updates
	require.Equal(t, "hi", got.Text)

	require.NoError(t, f.SendMessage(ctx, 1, "hello"))
	require.Len(t, f.Sent, 1)
	require.Equal(t, "hello", f.Sent[0].Text)
}
