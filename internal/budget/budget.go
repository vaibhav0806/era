package budget

import "strings"

type Profile struct {
	Name       string
	MaxIter    int
	MaxCents   int
	MaxWallSec int
}

// Profiles defines the three named budget presets. Caps are independent —
// a profile exceeding any one cap is considered exceeded.
var Profiles = map[string]Profile{
	"quick":   {"quick", 20, 5, 600},     // 10 min, 20 iters, $0.05
	"default": {"default", 60, 20, 1800}, // 30 min, 60 iters, $0.20
	"deep":    {"deep", 120, 100, 3600},  // 60 min, 120 iters, $1.00
}

// ParseBudgetFlag strips a leading `--budget=NAME` token from desc.
// Returns (profileName, cleanedDesc). Unknown profile names fall back to
// "default" with the description preserved as-is.
func ParseBudgetFlag(desc string) (string, string) {
	desc = strings.TrimSpace(desc)
	if !strings.HasPrefix(desc, "--budget=") {
		return "default", desc
	}
	end := strings.IndexByte(desc, ' ')
	if end < 0 {
		return "default", desc // malformed; no trailing space, preserve
	}
	name := strings.TrimPrefix(desc[:end], "--budget=")
	if _, ok := Profiles[name]; !ok {
		return "default", desc // unknown name, preserve whole desc
	}
	return name, strings.TrimSpace(desc[end+1:])
}
