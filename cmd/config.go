package cmd

import (
	"fmt"
	"os"

	"github.com/k1LoW/mo/internal/server"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type configFile struct {
	Port                         int           `yaml:"port"`
	Bind                         string        `yaml:"bind"`
	Foreground                   bool          `yaml:"foreground"`
	NoOpen                       bool          `yaml:"no-open"`
	NoRestart                    bool          `yaml:"no-restart"`
	NoDelete                     bool          `yaml:"no-delete"`
	NoFileMove                   bool          `yaml:"no-file-move"`
	NewFileNoAutoSelect          bool          `yaml:"newfile-no-autoselect"`
	ReadOnly                     bool          `yaml:"read-only"`
	Shareable                    bool          `yaml:"shareable"`
	TrueFilenames                bool          `yaml:"true-filenames"`
	DangerouslyAllowRemoteAccess bool          `yaml:"dangerously-allow-remote-access"`
	Quiet                        bool          `yaml:"quiet"`
	Groups                       []groupConfig `yaml:"groups"`
}

type groupConfig struct {
	Name  string   `yaml:"name"`
	Watch []string `yaml:"watch"`
	Files []string `yaml:"files"`
}

func loadConfigFile(path string) (*configFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}
	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return &cfg, nil
}

// applyConfig applies config file values to package-level vars,
// skipping any flag that was explicitly set on the CLI.
func applyConfig(cmd *cobra.Command, cfg *configFile) {
	if !cmd.Flags().Changed("port") && cfg.Port != 0 {
		port = cfg.Port
	}
	if !cmd.Flags().Changed("bind") && cfg.Bind != "" {
		bind = cfg.Bind
	}
	if !cmd.Flags().Changed("foreground") && cfg.Foreground {
		foreground = true
	}
	if !cmd.Flags().Changed("no-open") && cfg.NoOpen {
		noOpen = true
	}
	if !cmd.Flags().Changed("no-restart") && cfg.NoRestart {
		noRestart = true
	}
	if !cmd.Flags().Changed("no-delete") && cfg.NoDelete {
		noDelete = true
	}
	if !cmd.Flags().Changed("no-file-move") && cfg.NoFileMove {
		noFileMove = true
	}
	if !cmd.Flags().Changed("newfile-no-autoselect") && cfg.NewFileNoAutoSelect {
		noNewFileAutoSelect = true
	}
	if !cmd.Flags().Changed("read-only") && cfg.ReadOnly {
		readOnly = true
	}
	if !cmd.Flags().Changed("shareable") && cfg.Shareable {
		shareable = true
	}
	if !cmd.Flags().Changed("true-filenames") && cfg.TrueFilenames {
		trueFilenames = true
	}
	if !cmd.Flags().Changed("dangerously-allow-remote-access") && cfg.DangerouslyAllowRemoteAccess {
		dangerouslyAllowRemoteAccess = true
	}
	if !cmd.Flags().Changed("quiet") && cfg.Quiet {
		quiet = true
	}
}

// buildGroupsFromConfig converts the groups section of the config file
// into the filesByGroup and patternsByGroup maps used by startServer.
func buildGroupsFromConfig(cfg *configFile) (map[string][]string, map[string][]string, error) {
	filesByGroup := make(map[string][]string)
	patternsByGroup := make(map[string][]string)

	for _, g := range cfg.Groups {
		name, err := server.ResolveGroupName(g.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid group name %q: %w", g.Name, err)
		}

		files, err := resolveFiles(g.Files)
		if err != nil {
			return nil, nil, fmt.Errorf("group %q: %w", name, err)
		}
		if len(files) > 0 {
			filesByGroup[name] = files
		}

		patterns, err := resolvePatterns(g.Watch)
		if err != nil {
			return nil, nil, fmt.Errorf("group %q: %w", name, err)
		}
		if len(patterns) > 0 {
			patternsByGroup[name] = patterns
		}
	}

	return filesByGroup, patternsByGroup, nil
}
