package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubOps records calls instead of touching a real DB.
type stubOps struct {
	Created []string
	Status  map[int64]string
	Listed  bool
}

func (s *stubOps) CreateTask(ctx context.Context, desc string) (int64, error) {
	s.Created = append(s.Created, desc)
	return int64(len(s.Created)), nil
}
func (s *stubOps) TaskStatus(ctx context.Context, id int64) (string, error) {
	if v, ok := s.Status[id]; ok {
		return v, nil
	}
	return "", ErrTaskNotFound
}
func (s *stubOps) ListRecent(ctx context.Context, limit int) ([]TaskSummary, error) {
	s.Listed = true
	return []TaskSummary{{ID: 1, Description: "t1", Status: "queued"}}, nil
}

func TestHandler_TaskCommand(t *testing.T) {
	ops := &stubOps{}
	fc := NewFakeClient()
	h := NewHandler(fc, ops)

	err := h.Handle(context.Background(), Update{ChatID: 42, Text: "/task build auth flow"})
	require.NoError(t, err)
	require.Equal(t, []string{"build auth flow"}, ops.Created)
	require.Len(t, fc.Sent, 1)
	require.Contains(t, fc.Sent[0].Text, "queued")
}

func TestHandler_StatusCommand(t *testing.T) {
	ops := &stubOps{Status: map[int64]string{7: "running"}}
	fc := NewFakeClient()
	h := NewHandler(fc, ops)

	require.NoError(t, h.Handle(context.Background(), Update{ChatID: 1, Text: "/status 7"}))
	require.Contains(t, strings.ToLower(fc.Sent[0].Text), "running")
}

func TestHandler_StatusUnknownTask(t *testing.T) {
	ops := &stubOps{Status: map[int64]string{}}
	fc := NewFakeClient()
	h := NewHandler(fc, ops)
	require.NoError(t, h.Handle(context.Background(), Update{ChatID: 1, Text: "/status 99"}))
	require.Contains(t, fc.Sent[0].Text, "not found")
}

func TestHandler_ListCommand(t *testing.T) {
	ops := &stubOps{}
	fc := NewFakeClient()
	h := NewHandler(fc, ops)
	require.NoError(t, h.Handle(context.Background(), Update{ChatID: 1, Text: "/list"}))
	require.True(t, ops.Listed)
	require.Contains(t, fc.Sent[0].Text, "t1")
}

func TestHandler_UnknownCommand(t *testing.T) {
	ops := &stubOps{}
	fc := NewFakeClient()
	h := NewHandler(fc, ops)
	require.NoError(t, h.Handle(context.Background(), Update{ChatID: 1, Text: "/wat"}))
	require.Contains(t, strings.ToLower(fc.Sent[0].Text), "unknown")
}
