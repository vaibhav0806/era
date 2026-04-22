package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_AllRequiredPresent(t *testing.T) {
	t.Setenv("PI_TELEGRAM_TOKEN", "tok")
	t.Setenv("PI_TELEGRAM_ALLOWED_USER_ID", "12345")
	t.Setenv("PI_GITHUB_PAT", "ghp_xxx")
	t.Setenv("PI_GITHUB_SANDBOX_REPO", "vaibhavpandey/pi-agent-sandbox")
	t.Setenv("PI_DB_PATH", "./test.db")
	t.Setenv("PI_OPENROUTER_API_KEY", "sk-or-test")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "tok", cfg.TelegramToken)
	require.Equal(t, int64(12345), cfg.TelegramAllowedUserID)
	require.Equal(t, "ghp_xxx", cfg.GitHubPAT)
	require.Equal(t, "vaibhavpandey/pi-agent-sandbox", cfg.GitHubSandboxRepo)
	require.Equal(t, "./test.db", cfg.DBPath)
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("PI_TELEGRAM_TOKEN", "")
	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "PI_TELEGRAM_TOKEN")
}

func TestLoad_InvalidAllowedUserID(t *testing.T) {
	t.Setenv("PI_TELEGRAM_TOKEN", "tok")
	t.Setenv("PI_TELEGRAM_ALLOWED_USER_ID", "not-a-number")
	t.Setenv("PI_GITHUB_PAT", "x")
	t.Setenv("PI_GITHUB_SANDBOX_REPO", "x/y")
	t.Setenv("PI_DB_PATH", "x")
	t.Setenv("PI_OPENROUTER_API_KEY", "k")

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "PI_TELEGRAM_ALLOWED_USER_ID")
}

func TestLoad_WithOpenRouterAndCaps(t *testing.T) {
	t.Setenv("PI_TELEGRAM_TOKEN", "tok")
	t.Setenv("PI_TELEGRAM_ALLOWED_USER_ID", "12345")
	t.Setenv("PI_GITHUB_PAT", "ghp_x")
	t.Setenv("PI_GITHUB_SANDBOX_REPO", "a/b")
	t.Setenv("PI_DB_PATH", "./x.db")
	t.Setenv("PI_OPENROUTER_API_KEY", "sk-or-xxx")
	t.Setenv("PI_MODEL", "moonshotai/kimi-k2.5")
	t.Setenv("PI_MAX_TOKENS_PER_TASK", "100000")
	t.Setenv("PI_MAX_COST_CENTS_PER_TASK", "25")
	t.Setenv("PI_MAX_ITERATIONS_PER_TASK", "20")
	t.Setenv("PI_MAX_WALL_CLOCK_SECONDS", "600")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "sk-or-xxx", cfg.OpenRouterAPIKey)
	require.Equal(t, "moonshotai/kimi-k2.5", cfg.PiModel)
	require.Equal(t, 100000, cfg.MaxTokensPerTask)
	require.Equal(t, 25, cfg.MaxCostCentsPerTask)
	require.Equal(t, 20, cfg.MaxIterationsPerTask)
	require.Equal(t, 600, cfg.MaxWallClockSeconds)
}

func TestLoad_DefaultsForOptional(t *testing.T) {
	t.Setenv("PI_TELEGRAM_TOKEN", "tok")
	t.Setenv("PI_TELEGRAM_ALLOWED_USER_ID", "1")
	t.Setenv("PI_GITHUB_PAT", "x")
	t.Setenv("PI_GITHUB_SANDBOX_REPO", "a/b")
	t.Setenv("PI_DB_PATH", "./x.db")
	t.Setenv("PI_OPENROUTER_API_KEY", "k")
	// All PI_MAX_* and PI_MODEL unset — expect defaults.
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "moonshotai/kimi-k2.6", cfg.PiModel)
	require.Equal(t, 500000, cfg.MaxTokensPerTask)
	require.Equal(t, 50, cfg.MaxCostCentsPerTask)
	require.Equal(t, 30, cfg.MaxIterationsPerTask)
	require.Equal(t, 900, cfg.MaxWallClockSeconds)
}

func TestLoad_MissingOpenRouterKey(t *testing.T) {
	t.Setenv("PI_TELEGRAM_TOKEN", "tok")
	t.Setenv("PI_TELEGRAM_ALLOWED_USER_ID", "1")
	t.Setenv("PI_GITHUB_PAT", "x")
	t.Setenv("PI_GITHUB_SANDBOX_REPO", "a/b")
	t.Setenv("PI_DB_PATH", "./x.db")
	t.Setenv("PI_OPENROUTER_API_KEY", "")
	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "PI_OPENROUTER_API_KEY")
}
