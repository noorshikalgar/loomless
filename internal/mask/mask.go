// Package mask pattern-matches common secret shapes (API keys, tokens,
// private key blocks, "password=..."-style assignments) out of captured
// text before it's written to disk. This is the "basic secret-masking at
// capture time" Phase 1 roadmap item — see docs/product/07-edge-cases.md
// and decision #6 in docs/product/11-decisions.md.
//
// Scope: text only (terminal output, commands, window titles, notes) —
// not screenshot pixels. Doc calls out pixel-level detection as
// unreliable/out of scope for now (Phase 2's "auto-blur" is the visual
// equivalent). This will never catch everything: it's a safety net for
// common, recognizable secret shapes, not a guarantee. The doc is explicit
// that the human/agent reviewing before sharing is still the real
// backstop, not this.
package mask

import "regexp"

const placeholder = "«masked»"

type rule struct {
	name string
	re   *regexp.Regexp
	// repl is a ReplaceAllString template; "$0" (the whole match) is
	// replaced wholesale unless a rule needs to preserve part of the
	// match (e.g. keep the "password=" label, mask only the value).
	repl string
}

var rules = []rule{
	{
		name: "private_key_block",
		re:   regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
		repl: "-----BEGIN PRIVATE KEY----- " + placeholder + " -----END PRIVATE KEY-----",
	},
	{
		name: "aws_access_key_id",
		re:   regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		repl: placeholder,
	},
	{
		// Covers OpenAI/Anthropic/Stripe-style "sk-...", plus "pk-"/"rk-"
		// variants some providers use for public/restricted keys.
		name: "prefixed_api_key",
		re:   regexp.MustCompile(`\b(?:sk|pk|rk)-[A-Za-z0-9_-]{16,}\b`),
		repl: placeholder,
	},
	{
		name: "github_token",
		re:   regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`),
		repl: placeholder,
	},
	{
		name: "slack_token",
		re:   regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`),
		repl: placeholder,
	},
	{
		name: "jwt",
		re:   regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
		repl: placeholder,
	},
	{
		// "password=hunter2", "API_KEY: abc123", "Authorization: Bearer
		// xyz" — requires an explicit ":"/"=" separator, not just
		// whitespace: a bare-whitespace variant also matched ordinary
		// prose like "a secret plan for the roadmap" (caught by the test
		// suite), which made this rule too noisy to be usable. Keeps the
		// label so the reader still knows what was there, masks only the
		// value.
		name: "labeled_secret_assignment",
		re:   regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|auth[_-]?token|secret|password|passwd|pwd|bearer)\b(\s*[:=]\s*)['"]?([^\s'"]{4,})['"]?`),
		repl: "${1}${2}" + placeholder,
	},
	{
		// CLI-flag form doesn't need a ":"/"=" to be unambiguous — a
		// leading "--" already rules out ordinary prose.
		name: "cli_flag_secret",
		re:   regexp.MustCompile(`(?i)(--(?:api[_-]?key|access[_-]?token|auth[_-]?token|secret|password|passwd|pwd|token)(?:=|\s+))['"]?([^\s'"]{4,})['"]?`),
		repl: "${1}" + placeholder,
	},
}

// Text masks every recognized secret shape in s, returning the masked
// text and the names of rules that matched (empty if nothing did) — the
// rule names are safe to log/display since they never contain the
// matched content itself.
func Text(s string) (masked string, matchedRules []string) {
	masked = s
	for _, r := range rules {
		if r.re.MatchString(masked) {
			matchedRules = append(matchedRules, r.name)
			masked = r.re.ReplaceAllString(masked, r.repl)
		}
	}
	return masked, matchedRules
}
