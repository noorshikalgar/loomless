package mask

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantRule    string
		wantMasked  bool
		mustContain string // substring that must survive (e.g. a label)
	}{
		{
			name:       "aws access key",
			input:      "export AWS_ACCESS_KEY_ID=AKIAABCDEFGHIJKLMNOP",
			wantRule:   "aws_access_key_id",
			wantMasked: true,
		},
		{
			name:       "openai-style key",
			input:      "curl -H 'Authorization: Bearer sk-proj-abcdefghijklmnopqrstuvwx'",
			wantRule:   "prefixed_api_key",
			wantMasked: true,
		},
		{
			name:       "github token",
			input:      "ghp_1234567890abcdefghijklmnopqrstuvwxyz12",
			wantRule:   "github_token",
			wantMasked: true,
		},
		{
			name:       "slack token",
			input:      "xoxb-FAKE-TEST-TOKEN-NOT-REAL-abcdefgh",
			wantRule:   "slack_token",
			wantMasked: true,
		},
		{
			name:       "jwt",
			input:      "token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			wantRule:   "jwt",
			wantMasked: true,
		},
		{
			name:        "labeled password preserves label",
			input:       "password=hunter22",
			wantRule:    "labeled_secret_assignment",
			wantMasked:  true,
			mustContain: "password=",
		},
		{
			name:        "labeled api key with colon and quotes",
			input:       `api_key: "abcd1234efgh5678"`,
			wantRule:    "labeled_secret_assignment",
			wantMasked:  true,
			mustContain: "api_key",
		},
		{
			name:        "cli flag with space-separated value",
			input:       "curl --password hunter22 https://example.com",
			wantRule:    "cli_flag_secret",
			wantMasked:  true,
			mustContain: "--password",
		},
		{
			name:       "private key block",
			input:      "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
			wantRule:   "private_key_block",
			wantMasked: true,
		},
		{
			name:       "plain benign text is untouched",
			input:      "clicked the Export button and downloaded the CSV",
			wantMasked: false,
		},
		{
			name:       "short word 'secret' alone does not misfire",
			input:      "this is a secret plan for the roadmap",
			wantMasked: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			masked, rules := Text(c.input)

			if c.wantMasked && len(rules) == 0 {
				t.Fatalf("Text(%q) matched no rules, want at least one", c.input)
			}
			if !c.wantMasked && len(rules) != 0 {
				t.Fatalf("Text(%q) matched rules %v, want none", c.input, rules)
			}
			if c.wantRule != "" {
				found := false
				for _, r := range rules {
					if r == c.wantRule {
						found = true
					}
				}
				if !found {
					t.Errorf("Text(%q) matched rules %v, want %q among them", c.input, rules, c.wantRule)
				}
			}
			if c.wantMasked && masked == c.input {
				t.Errorf("Text(%q) returned input unchanged despite reporting a match", c.input)
			}
			if c.mustContain != "" && !strings.Contains(masked, c.mustContain) {
				t.Errorf("Text(%q) = %q, want it to still contain %q", c.input, masked, c.mustContain)
			}
			if c.wantMasked && strings.Contains(masked, "AKIAABCDEFGHIJKLMNOP") {
				t.Errorf("Text(%q) = %q, raw secret leaked into masked output", c.input, masked)
			}
		})
	}
}
