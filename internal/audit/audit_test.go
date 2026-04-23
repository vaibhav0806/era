package audit_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/audit"
)

func TestParse_ValidAuditLine(t *testing.T) {
	line := `AUDIT {"time":"2026-04-23T10:00:00Z","method":"GET","path":"/health","host":"","status":200,"bytes":3,"latency_ms":1}`
	e, ok := audit.Parse(line)
	require.True(t, ok)
	require.Equal(t, "GET", e.Method)
	require.Equal(t, "/health", e.Path)
	require.Equal(t, 200, e.Status)
}

func TestParse_NonAuditLineRejected(t *testing.T) {
	_, ok := audit.Parse("INFO regular log line")
	require.False(t, ok)
}

func TestParse_MalformedJSONRejected(t *testing.T) {
	_, ok := audit.Parse("AUDIT {not-json")
	require.False(t, ok)
}

func TestStreamAndCollect(t *testing.T) {
	r := strings.NewReader(`runner clone start
AUDIT {"time":"2026-04-23T10:00:00Z","method":"GET","path":"/health","status":200,"bytes":3,"latency_ms":0}
AUDIT {"time":"2026-04-23T10:00:01Z","method":"CONNECT","host":"github.com","path":"","status":200,"bytes":0,"latency_ms":12}
runner clone done
`)
	collected := []audit.Entry{}
	err := audit.Stream(r, func(e audit.Entry) { collected = append(collected, e) })
	require.NoError(t, err)
	require.Len(t, collected, 2)
	require.Equal(t, "/health", collected[0].Path)
	require.Equal(t, "github.com", collected[1].Host)
}
