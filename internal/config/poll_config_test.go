package config

import (
	"encoding/json"
	"errors"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestPollConfig_Validate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		config   PollConfig
		expected error
	}{
		{
			name: "Valid config",
			config: PollConfig{
				CloneUrl:  "https://example.com/repo.git",
				Reference: "main",
				Interval:  10,
			},
			expected: nil,
		},
		{
			name: "Invalid config - empty CloneUrl",
			config: PollConfig{
				CloneUrl:  "",
				Reference: "main",
				Interval:  10,
			},
			expected: ErrKeyNotFound,
		},
		{
			name: "Invalid config - empty Reference",
			config: PollConfig{
				CloneUrl:  "https://example.com/repo.git",
				Reference: "",
				Interval:  10,
			},
			expected: ErrKeyNotFound,
		},
		{
			name: "Invalid config - negative Interval",
			config: PollConfig{
				CloneUrl:  "https://example.com/repo.git",
				Reference: "main",
				Interval:  -5,
			},
			expected: ErrPollIntervalTooLow,
		},
		{
			name: "Invalid config - 5s Interval",
			config: PollConfig{
				CloneUrl:  "https://example.com/repo.git",
				Reference: "main",
				Interval:  5,
			},
			expected: ErrPollIntervalTooLow,
		},
		{
			name: "Invalid config - zero Interval (Disabled)",
			config: PollConfig{
				CloneUrl:  "https://example.com/repo.git",
				Reference: "main",
				Interval:  0,
			},
			expected: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()
			if !errors.Is(err, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, err)
			}
		})
	}
}

// TestInlineDeployment_ExplicitFalseBoolsPreserved is a regression test for the
// creasty/defaults behavior where Go's bool zero-value is indistinguishable from
// an explicit `false`. PollConfig.Validate() previously called defaults.Set() on
// each inline DeployConfig after YAML parsing, silently flipping any bool field
// with `default:"true"` back to true.
func TestInlineDeployment_ExplicitFalseBoolsPreserved(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		yaml  string
		check func(t *testing.T, d *DeployConfig)
	}{
		{
			name: "prune_images false",
			yaml: `
url: https://example.com/repo.git
reference: refs/heads/main
interval: 60
deployments:
  - name: stack
    prune_images: false
`,
			check: func(t *testing.T, d *DeployConfig) {
				if d.PruneImages {
					t.Errorf("PruneImages: expected false, got true")
				}
			},
		},
		{
			name: "remove_orphans false",
			yaml: `
url: https://example.com/repo.git
reference: refs/heads/main
interval: 60
deployments:
  - name: stack
    remove_orphans: false
`,
			check: func(t *testing.T, d *DeployConfig) {
				if d.RemoveOrphans {
					t.Errorf("RemoveOrphans: expected false, got true")
				}
			},
		},
		{
			name: "reconciliation.enabled false",
			yaml: `
url: https://example.com/repo.git
reference: refs/heads/main
interval: 60
deployments:
  - name: stack
    reconciliation:
      enabled: false
`,
			check: func(t *testing.T, d *DeployConfig) {
				if d.Reconciliation.Enabled {
					t.Errorf("Reconciliation.Enabled: expected false, got true")
				}
			},
		},
		{
			name: "destroy_opts all false",
			yaml: `
url: https://example.com/repo.git
reference: refs/heads/main
interval: 60
deployments:
  - name: stack
    destroy_opts:
      remove_volumes: false
      remove_images: false
      remove_dir: false
`,
			check: func(t *testing.T, d *DeployConfig) {
				if d.DestroyOpts.RemoveVolumes {
					t.Errorf("DestroyOpts.RemoveVolumes: expected false, got true")
				}
				if d.DestroyOpts.RemoveImages {
					t.Errorf("DestroyOpts.RemoveImages: expected false, got true")
				}
				if d.DestroyOpts.RemoveRepoDir {
					t.Errorf("DestroyOpts.RemoveRepoDir: expected false, got true")
				}
			},
		},
		{
			name: "auto_discover_opts.delete false",
			yaml: `
url: https://example.com/repo.git
reference: refs/heads/main
interval: 60
deployments:
  - name: stack
    auto_discover: true
    auto_discover_opts:
      delete: false
`,
			check: func(t *testing.T, d *DeployConfig) {
				if d.AutoDiscoverOpts.Delete {
					t.Errorf("AutoDiscoverOpts.Delete: expected false, got true")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var pc PollConfig
			if err := yaml.Unmarshal([]byte(tc.yaml), &pc); err != nil {
				t.Fatalf("yaml unmarshal: %v", err)
			}

			if err := pc.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}

			if len(pc.Deployments) != 1 {
				t.Fatalf("expected 1 deployment, got %d", len(pc.Deployments))
			}

			tc.check(t, pc.Deployments[0])
		})
	}
}

// TestInlineDeployment_JSONExplicitFalseBoolsPreserved covers the JSON unmarshal
// path. It exercises DeployConfig.UnmarshalJSON, which must apply defaults
// before json.Unmarshal so explicit false values stick.
func TestInlineDeployment_JSONExplicitFalseBoolsPreserved(t *testing.T) {
	t.Parallel()

	body := `{
		"url": "https://example.com/repo.git",
		"reference": "refs/heads/main",
		"interval": 60,
		"deployments": [
			{"name": "stack", "prune_images": false, "reconciliation": {"enabled": false}}
		]
	}`

	var pc PollConfig
	if err := json.Unmarshal([]byte(body), &pc); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if err := pc.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	if len(pc.Deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(pc.Deployments))
	}

	d := pc.Deployments[0]

	if d.PruneImages {
		t.Errorf("PruneImages: expected false, got true")
	}

	if d.Reconciliation.Enabled {
		t.Errorf("Reconciliation.Enabled: expected false, got true")
	}

	// Sanity: an unset bool with default:"true" should still be true after defaults applied.
	if !d.RemoveOrphans {
		t.Errorf("RemoveOrphans: expected default true, got false")
	}
}

func TestPollConfig_String(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		config   PollConfig
		expected string
	}{
		{
			name: "Valid config",
			config: PollConfig{
				CloneUrl:  "https://example.com/repo.git",
				Reference: "main",
				Interval:  10,
			},
			expected: "PollConfig{CloneUrl: https://example.com/repo.git, Reference: main, Interval: 10}",
		},
		{
			name: "Basic config",
			config: PollConfig{
				CloneUrl:  "https://example.com/repo.git",
				Reference: "main",
				Interval:  180,
			},
			expected: "PollConfig{CloneUrl: https://example.com/repo.git, Reference: main, Interval: 180}",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := tc.config.String()
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}
