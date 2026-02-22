package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent       AgentConfig  `yaml:"agent"`
	Settings    Settings     `yaml:"settings"`
	Stations    []Station    `yaml:"stations"`
	Gates       []Gate       `yaml:"gates,omitempty"`
	Permissions *Permissions `yaml:"permissions,omitempty"`
	Preamble    string       `yaml:"preamble,omitempty"`
}

// Gate defines a pre-commit quality gate (linter, formatter, type checker, etc.).
type Gate struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
}

// Permissions mirrors the Claude Code .claude/settings.json permissions block.
// When set, line writes this into each worktree before invoking the agent.
type Permissions struct {
	Allow []string `yaml:"allow" json:"allow"`
	Deny  []string `yaml:"deny,omitempty" json:"deny,omitempty"`
}

type AgentConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type Settings struct {
	BranchPrefix string `yaml:"branch_prefix"`
	Watches      string `yaml:"watches"`
}

type Station struct {
	Name     string   `yaml:"name"`
	Watches  string   `yaml:"watches"`
	Prompt   string   `yaml:"prompt"`
	Command  string   `yaml:"command,omitempty"`
	Args     []string `yaml:"args,omitempty"`
	Preamble string   `yaml:"preamble,omitempty"`
}

// DefaultPreamble is the preamble prepended to every station prompt when no
// custom preamble is configured.
const DefaultPreamble = "You are running non-interactively. Do not ask questions or wait for confirmation.\nIf something is unclear, make your best judgement and proceed.\nDo not run git commit — your changes will be committed automatically."

// ResolvePreamble returns the effective preamble for a station.
// Per-station preamble takes priority, then global config preamble, then DefaultPreamble.
func (cfg *Config) ResolvePreamble(c Station) string {
	if c.Preamble != "" {
		return c.Preamble
	}
	if cfg.Preamble != "" {
		return cfg.Preamble
	}
	return DefaultPreamble
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return parse(data)
}

func parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if cfg.Settings.BranchPrefix == "" {
		cfg.Settings.BranchPrefix = "line/"
	}
	if cfg.Settings.Watches == "" {
		cfg.Settings.Watches = "main"
	}

	// Populate Watches from position: first station watches settings.watches,
	// each subsequent station watches the previous one.
	for i := range cfg.Stations {
		if i == 0 {
			cfg.Stations[i].Watches = cfg.Settings.Watches
		} else {
			cfg.Stations[i].Watches = cfg.Stations[i-1].Name
		}
	}

	return &cfg, nil
}

func Validate(cfg *Config) []error {
	var errs []error

	if cfg.Agent.Command == "" {
		errs = append(errs, fmt.Errorf("agent.command is required"))
	}

	if len(cfg.Stations) == 0 {
		errs = append(errs, fmt.Errorf("at least one station is required"))
	}

	names := make(map[string]bool)
	for i, c := range cfg.Stations {
		if c.Name == "" {
			errs = append(errs, fmt.Errorf("stations[%d]: name is required", i))
		} else if names[c.Name] {
			errs = append(errs, fmt.Errorf("stations[%d]: duplicate name %q", i, c.Name))
		} else {
			names[c.Name] = true
		}

		if c.Prompt == "" {
			errs = append(errs, fmt.Errorf("stations[%d] (%s): prompt is required", i, c.Name))
		}
	}

	errs = append(errs, ValidateGates(cfg.Gates)...)

	return errs
}

// ValidateGates checks that all gates have non-empty names and run commands,
// and that gate names are unique.
func ValidateGates(gates []Gate) []error {
	var errs []error
	names := make(map[string]bool)
	for i, g := range gates {
		if g.Name == "" {
			errs = append(errs, fmt.Errorf("gates[%d]: name is required", i))
		} else if names[g.Name] {
			errs = append(errs, fmt.Errorf("gates[%d]: duplicate name %q", i, g.Name))
		} else {
			names[g.Name] = true
		}
		if g.Run == "" {
			errs = append(errs, fmt.Errorf("gates[%d]: run is required", i))
		}
	}
	return errs
}

// HasStation returns true if a station with the given name exists in the config.
func (cfg *Config) HasStation(name string) bool {
	for _, c := range cfg.Stations {
		if c.Name == name {
			return true
		}
	}
	return false
}

// ValidateStationName returns an error if the station name does not exist in the config.
func (cfg *Config) ValidateStationName(name string) error {
	if !cfg.HasStation(name) {
		return fmt.Errorf("unknown station %q", name)
	}
	return nil
}

