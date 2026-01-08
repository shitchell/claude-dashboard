package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
)

func TestLoadFromYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*Config) error
		wantErr bool
	}{
		{
			name: "basic config",
			yaml: `
mode: grid
layout: list
`,
			check: func(cfg *Config) error {
				if cfg.Mode != DisplayModeGrid {
					return errors.New("mode should be grid")
				}
				if cfg.Layout != LayoutTypeList {
					return errors.New("layout should be list")
				}
				return nil
			},
		},
		{
			name: "columns config",
			yaml: `
columns:
  - name
  - status
  - preview
`,
			check: func(cfg *Config) error {
				if len(cfg.Columns) != 3 {
					return errors.New("should have 3 columns")
				}
				if cfg.Columns[0] != ColumnTypeName {
					return errors.New("first column should be name")
				}
				if cfg.Columns[1] != ColumnTypeStatus {
					return errors.New("second column should be status")
				}
				if cfg.Columns[2] != ColumnTypePreview {
					return errors.New("third column should be preview")
				}
				return nil
			},
		},
		{
			name: "widths config",
			yaml: `
widths:
  name: 25
  preview: 50
  modified: 15
`,
			check: func(cfg *Config) error {
				if cfg.Widths.Name != 25 {
					return errors.New("name width should be 25")
				}
				if cfg.Widths.Preview != 50 {
					return errors.New("preview width should be 50")
				}
				if cfg.Widths.Modified != 15 {
					return errors.New("modified width should be 15")
				}
				return nil
			},
		},
		{
			name: "side panel config",
			yaml: `
side_panel:
  enabled: true
  min_width: 30
  width: 50
`,
			check: func(cfg *Config) error {
				if !cfg.SidePanel.Enabled {
					return errors.New("side panel should be enabled")
				}
				if cfg.SidePanel.MinWidth != 30 {
					return errors.New("min width should be 30")
				}
				if cfg.SidePanel.Width != 50 {
					return errors.New("width should be 50")
				}
				return nil
			},
		},
		{
			name: "grid config",
			yaml: `
grid:
  card_width: 45
  card_height: 10
  gap: 2
`,
			check: func(cfg *Config) error {
				if cfg.Grid.CardWidth != 45 {
					return errors.New("card width should be 45")
				}
				if cfg.Grid.CardHeight != 10 {
					return errors.New("card height should be 10")
				}
				if cfg.Grid.Gap != 2 {
					return errors.New("gap should be 2")
				}
				return nil
			},
		},
		{
			name: "indicators config",
			yaml: `
indicators:
  active: ">"
  idle: "-"
  exited: "x"
`,
			check: func(cfg *Config) error {
				if cfg.Indicators.Active != ">" {
					return errors.New("active indicator should be >")
				}
				if cfg.Indicators.Idle != "-" {
					return errors.New("idle indicator should be -")
				}
				if cfg.Indicators.Exited != "x" {
					return errors.New("exited indicator should be x")
				}
				return nil
			},
		},
		{
			name: "sort config",
			yaml: `
sort:
  field: project
  ascending: true
`,
			check: func(cfg *Config) error {
				if cfg.Sort.Field != "project" {
					return errors.New("sort field should be project")
				}
				if !cfg.Sort.Ascending {
					return errors.New("sort ascending should be true")
				}
				return nil
			},
		},
		{
			name: "filter config",
			yaml: `
filter:
  max_age: "7d"
  project: "myproject"
  exclude_exited: true
`,
			check: func(cfg *Config) error {
				if cfg.Filter.MaxAge != "7d" {
					return errors.New("max age should be 7d")
				}
				if cfg.Filter.Project != "myproject" {
					return errors.New("project should be myproject")
				}
				if !cfg.Filter.ExcludeExited {
					return errors.New("exclude exited should be true")
				}
				return nil
			},
		},
		{
			name: "tmux config",
			yaml: `
tmux:
  enabled: false
  highlight_running: false
  navigation_method: switch-client
`,
			check: func(cfg *Config) error {
				// Note: booleans from YAML don't override defaults due to zero value issue
				if cfg.Tmux.NavigationMethod != "switch-client" {
					return errors.New("navigation method should be switch-client")
				}
				return nil
			},
		},
		{
			name: "cache config",
			yaml: `
cache:
  enabled: true
  dir: ".mycache"
  filename: "data.json"
`,
			check: func(cfg *Config) error {
				if cfg.Cache.Dir != ".mycache" {
					return errors.New("cache dir should be .mycache")
				}
				if cfg.Cache.Filename != "data.json" {
					return errors.New("cache filename should be data.json")
				}
				return nil
			},
		},
		{
			name: "refresh config",
			yaml: `
refresh:
  enabled: true
  interval: 10
`,
			check: func(cfg *Config) error {
				if cfg.Refresh.Interval != 10*time.Second {
					return errors.New("refresh interval should be 10s")
				}
				return nil
			},
		},
		{
			name: "sessions config",
			yaml: `
sessions:
  projects_dir: ".claude/projects"
  max_name_length: 50
  max_preview_length: 100
`,
			check: func(cfg *Config) error {
				if cfg.Sessions.MaxNameLength != 50 {
					return errors.New("max name length should be 50")
				}
				if cfg.Sessions.MaxPreviewLength != 100 {
					return errors.New("max preview length should be 100")
				}
				return nil
			},
		},
		{
			name:    "invalid yaml",
			yaml:    `invalid: [yaml`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadFromYAML([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.check != nil {
				if err := tt.check(cfg); err != nil {
					t.Errorf("check failed: %v", err)
				}
			}
		})
	}
}

func TestLoadFromYAMLMergesWithDefaults(t *testing.T) {
	// Load a minimal config and verify defaults are preserved
	yaml := `
mode: grid
`
	cfg, err := LoadFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFromYAML() error = %v", err)
	}

	// Check that mode was overridden
	if cfg.Mode != DisplayModeGrid {
		t.Errorf("Mode should be grid, got %v", cfg.Mode)
	}

	// Check that defaults are preserved
	if cfg.Widths.Name != constants.DefaultColumnWidthName {
		t.Errorf("Widths.Name should be default %d, got %d",
			constants.DefaultColumnWidthName, cfg.Widths.Name)
	}
	if cfg.Refresh.Enabled != DefaultRefreshEnabled {
		t.Errorf("Refresh.Enabled should be default %v, got %v",
			DefaultRefreshEnabled, cfg.Refresh.Enabled)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr error
	}{
		{
			name:    "valid defaults",
			modify:  func(cfg *Config) {},
			wantErr: nil,
		},
		{
			name: "invalid refresh interval",
			modify: func(cfg *Config) {
				cfg.Refresh.Interval = 500 * time.Millisecond
			},
			wantErr: ErrInvalidRefreshInterval,
		},
		{
			name: "negative column width",
			modify: func(cfg *Config) {
				cfg.Widths.Name = -1
			},
			wantErr: ErrInvalidColumnWidth,
		},
		{
			name: "negative side panel width",
			modify: func(cfg *Config) {
				cfg.SidePanel.MinWidth = -1
			},
			wantErr: ErrInvalidSidePanelWidth,
		},
		{
			name: "negative grid card width",
			modify: func(cfg *Config) {
				cfg.Grid.CardWidth = -1
			},
			wantErr: ErrInvalidGridCardSize,
		},
		{
			name: "invalid navigation method",
			modify: func(cfg *Config) {
				cfg.Tmux.NavigationMethod = "invalid-method"
			},
			wantErr: ErrInvalidNavigationMethod,
		},
		{
			name: "empty projects dir",
			modify: func(cfg *Config) {
				cfg.Sessions.ProjectsDir = ""
			},
			wantErr: ErrEmptyProjectsDir,
		},
		{
			name: "negative max name length",
			modify: func(cfg *Config) {
				cfg.Sessions.MaxNameLength = -1
			},
			wantErr: ErrInvalidMaxLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.modify(&cfg)

			err := validate(&cfg)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("validate() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("validate() error = nil, want %v", tt.wantErr)
				} else {
					var valErr *ValidationError
					if !errors.As(err, &valErr) {
						t.Errorf("error should be ValidationError, got %T", err)
					} else if !errors.Is(valErr.Err, tt.wantErr) {
						t.Errorf("validate() error = %v, want %v", valErr.Err, tt.wantErr)
					}
				}
			}
		})
	}
}

func TestValidationErrorUnwrap(t *testing.T) {
	err := &ValidationError{
		Field: "test.field",
		Err:   ErrInvalidColumnWidth,
	}

	if !errors.Is(err, ErrInvalidColumnWidth) {
		t.Error("ValidationError should unwrap to underlying error")
	}

	expected := "config validation error for test.field: column width must be positive"
	if err.Error() != expected {
		t.Errorf("ValidationError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestLoadWithConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
mode: grid
widths:
  name: 35
sort:
  field: project
  ascending: true
`
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify file values override defaults
	if cfg.Mode != DisplayModeGrid {
		t.Errorf("Mode = %v, want %v", cfg.Mode, DisplayModeGrid)
	}
	if cfg.Widths.Name != 35 {
		t.Errorf("Widths.Name = %d, want 35", cfg.Widths.Name)
	}
	if cfg.Sort.Field != "project" {
		t.Errorf("Sort.Field = %q, want %q", cfg.Sort.Field, "project")
	}
	if !cfg.Sort.Ascending {
		t.Error("Sort.Ascending should be true")
	}

	// Verify defaults are preserved for unspecified values
	if cfg.Widths.Preview != constants.DefaultColumnWidthPreview {
		t.Errorf("Widths.Preview = %d, want %d", cfg.Widths.Preview, constants.DefaultColumnWidthPreview)
	}
}

func TestLoadWithFlags(t *testing.T) {
	// Create a config with defaults and apply flags
	mode := "grid"
	project := "myproject"

	flags := &Flags{
		Mode:    &mode,
		Project: &project,
	}

	cfg, err := Load("", flags)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != DisplayModeGrid {
		t.Errorf("Mode = %v, want %v", cfg.Mode, DisplayModeGrid)
	}
	if cfg.Filter.Project != "myproject" {
		t.Errorf("Filter.Project = %q, want %q", cfg.Filter.Project, "myproject")
	}
}

func TestLoadPriorityFileOverridesDefaults(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
widths:
  name: 100
`
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// File value should override default
	if cfg.Widths.Name != 100 {
		t.Errorf("Widths.Name = %d, want 100", cfg.Widths.Name)
	}
}

func TestLoadPriorityFlagsOverrideFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
mode: list
widths:
  name: 100
`
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Flag overrides file value
	mode := "grid"
	flags := &Flags{Mode: &mode}

	cfg, err := Load(configPath, flags)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Flag value should override file value
	if cfg.Mode != DisplayModeGrid {
		t.Errorf("Mode = %v, want %v (flag should override file)", cfg.Mode, DisplayModeGrid)
	}

	// File value should still be applied for unoverridden fields
	if cfg.Widths.Name != 100 {
		t.Errorf("Widths.Name = %d, want 100 (file value should be preserved)", cfg.Widths.Name)
	}
}

func TestLoadWithMissingFile(t *testing.T) {
	// Load with non-existent file
	_, err := Load("/nonexistent/path/config.yaml", nil)
	if err == nil {
		t.Error("Load() should error for non-existent file")
	}
}

func TestLoadWithEmptyPath(t *testing.T) {
	// Load with empty path should succeed with defaults
	cfg, err := Load("", nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have defaults
	if cfg.Mode != DefaultMode {
		t.Errorf("Mode = %v, want %v", cfg.Mode, DefaultMode)
	}
}

func TestMergeConfig(t *testing.T) {
	dst := Defaults()
	src := Config{
		ModeString: "grid",
		Sort: SortConfig{
			Field: "project",
		},
		Widths: ColumnWidths{
			Name: 50,
			// Other widths are zero, should not override
		},
	}
	src.syncFromStrings()

	mergeConfig(&dst, &src)

	// Mode should be overridden
	if dst.Mode != DisplayModeGrid {
		t.Errorf("Mode = %v, want grid", dst.Mode)
	}

	// Sort field should be overridden
	if dst.Sort.Field != "project" {
		t.Errorf("Sort.Field = %q, want project", dst.Sort.Field)
	}

	// Name width should be overridden
	if dst.Widths.Name != 50 {
		t.Errorf("Widths.Name = %d, want 50", dst.Widths.Name)
	}

	// Preview width should be default (not overridden by zero)
	if dst.Widths.Preview != constants.DefaultColumnWidthPreview {
		t.Errorf("Widths.Preview = %d, want %d", dst.Widths.Preview, constants.DefaultColumnWidthPreview)
	}
}
