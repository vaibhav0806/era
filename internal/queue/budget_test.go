package queue_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/queue"
)

func TestProfiles_KnownNames(t *testing.T) {
	require.Contains(t, queue.Profiles, "quick")
	require.Contains(t, queue.Profiles, "default")
	require.Contains(t, queue.Profiles, "deep")
	require.Equal(t, 60, queue.Profiles["default"].MaxIter)
	require.Equal(t, 20, queue.Profiles["default"].MaxCents)
	require.Equal(t, 1800, queue.Profiles["default"].MaxWallSec)
}

func TestParseBudgetFlag_NoFlag(t *testing.T) {
	profile, desc := queue.ParseBudgetFlag("build something")
	require.Equal(t, "default", profile)
	require.Equal(t, "build something", desc)
}

func TestParseBudgetFlag_DeepFlag(t *testing.T) {
	profile, desc := queue.ParseBudgetFlag("--budget=deep build a complex thing")
	require.Equal(t, "deep", profile)
	require.Equal(t, "build a complex thing", desc)
}

func TestParseBudgetFlag_QuickFlag(t *testing.T) {
	profile, desc := queue.ParseBudgetFlag("--budget=quick foo")
	require.Equal(t, "quick", profile)
	require.Equal(t, "foo", desc)
}

func TestParseBudgetFlag_UnknownProfile(t *testing.T) {
	profile, desc := queue.ParseBudgetFlag("--budget=hyperultra do thing")
	require.Equal(t, "default", profile)
	require.Equal(t, "--budget=hyperultra do thing", desc, "unknown profile preserved in desc")
}

func TestParseBudgetFlag_NoSpaceAfter(t *testing.T) {
	profile, desc := queue.ParseBudgetFlag("--budget=deep")
	require.Equal(t, "default", profile)
	require.Equal(t, "--budget=deep", desc, "malformed (no trailing desc) — preserve")
}

func TestParseBudgetFlag_LeadingWhitespace(t *testing.T) {
	profile, desc := queue.ParseBudgetFlag("   --budget=deep do thing")
	require.Equal(t, "deep", profile)
	require.Equal(t, "do thing", desc)
}

func TestParseBudgetFlag_FlagInMiddle_NotMatched(t *testing.T) {
	// Flag must be the FIRST token; later --budget= in description is preserved.
	profile, desc := queue.ParseBudgetFlag("build a thing --budget=deep should be in desc")
	require.Equal(t, "default", profile)
	require.Equal(t, "build a thing --budget=deep should be in desc", desc)
}
