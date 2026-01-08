package config

import (
	"testing"
	"time"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		check   func(*Flags) error
		wantErr bool
	}{
		{
			name: "empty args",
			args: []string{},
			check: func(f *Flags) error {
				if !f.IsEmpty() {
					return errorf("flags should be empty")
				}
				return nil
			},
		},
		{
			name: "config path short",
			args: []string{"-c", "/path/to/config.yaml"},
			check: func(f *Flags) error {
				if f.ConfigPath != "/path/to/config.yaml" {
					return errorf("ConfigPath = %q, want /path/to/config.yaml", f.ConfigPath)
				}
				return nil
			},
		},
		{
			name: "config path long",
			args: []string{"--config", "/path/to/config.yaml"},
			check: func(f *Flags) error {
				if f.ConfigPath != "/path/to/config.yaml" {
					return errorf("ConfigPath = %q, want /path/to/config.yaml", f.ConfigPath)
				}
				return nil
			},
		},
		{
			name: "mode short",
			args: []string{"-m", "grid"},
			check: func(f *Flags) error {
				if f.Mode == nil || *f.Mode != "grid" {
					return errorf("Mode should be grid")
				}
				return nil
			},
		},
		{
			name: "mode long",
			args: []string{"--mode", "list"},
			check: func(f *Flags) error {
				if f.Mode == nil || *f.Mode != "list" {
					return errorf("Mode should be list")
				}
				return nil
			},
		},
		{
			name: "layout short",
			args: []string{"-l", "grid"},
			check: func(f *Flags) error {
				if f.Layout == nil || *f.Layout != "grid" {
					return errorf("Layout should be grid")
				}
				return nil
			},
		},
		{
			name: "layout long",
			args: []string{"--layout", "list"},
			check: func(f *Flags) error {
				if f.Layout == nil || *f.Layout != "list" {
					return errorf("Layout should be list")
				}
				return nil
			},
		},
		{
			name: "columns",
			args: []string{"--columns", "name,status,preview"},
			check: func(f *Flags) error {
				if f.Columns == nil || *f.Columns != "name,status,preview" {
					return errorf("Columns should be name,status,preview")
				}
				return nil
			},
		},
		{
			name: "refresh",
			args: []string{"--refresh", "10s"},
			check: func(f *Flags) error {
				if f.RefreshInterval == nil || *f.RefreshInterval != 10*time.Second {
					return errorf("RefreshInterval should be 10s")
				}
				return nil
			},
		},
		{
			name: "no-refresh",
			args: []string{"--no-refresh"},
			check: func(f *Flags) error {
				if f.RefreshEnabled == nil || *f.RefreshEnabled != false {
					return errorf("RefreshEnabled should be false")
				}
				return nil
			},
		},
		{
			name: "no-tmux",
			args: []string{"--no-tmux"},
			check: func(f *Flags) error {
				if f.TmuxEnabled == nil || *f.TmuxEnabled != false {
					return errorf("TmuxEnabled should be false")
				}
				return nil
			},
		},
		{
			name: "no-cache",
			args: []string{"--no-cache"},
			check: func(f *Flags) error {
				if f.CacheEnabled == nil || *f.CacheEnabled != false {
					return errorf("CacheEnabled should be false")
				}
				return nil
			},
		},
		{
			name: "sort",
			args: []string{"--sort", "project"},
			check: func(f *Flags) error {
				if f.SortField == nil || *f.SortField != "project" {
					return errorf("SortField should be project")
				}
				return nil
			},
		},
		{
			name: "asc",
			args: []string{"--asc"},
			check: func(f *Flags) error {
				if f.SortAscending == nil || *f.SortAscending != true {
					return errorf("SortAscending should be true")
				}
				return nil
			},
		},
		{
			name: "project short",
			args: []string{"-p", "myproject"},
			check: func(f *Flags) error {
				if f.Project == nil || *f.Project != "myproject" {
					return errorf("Project should be myproject")
				}
				return nil
			},
		},
		{
			name: "project long",
			args: []string{"--project", "myproject"},
			check: func(f *Flags) error {
				if f.Project == nil || *f.Project != "myproject" {
					return errorf("Project should be myproject")
				}
				return nil
			},
		},
		{
			name: "max-age",
			args: []string{"--max-age", "7d"},
			check: func(f *Flags) error {
				if f.MaxAge == nil || *f.MaxAge != "7d" {
					return errorf("MaxAge should be 7d")
				}
				return nil
			},
		},
		{
			name: "running",
			args: []string{"--running"},
			check: func(f *Flags) error {
				if f.ExcludeExited == nil || *f.ExcludeExited != true {
					return errorf("ExcludeExited should be true")
				}
				return nil
			},
		},
		{
			name: "version short",
			args: []string{"-v"},
			check: func(f *Flags) error {
				if !f.Version {
					return errorf("Version should be true")
				}
				return nil
			},
		},
		{
			name: "version long",
			args: []string{"--version"},
			check: func(f *Flags) error {
				if !f.Version {
					return errorf("Version should be true")
				}
				return nil
			},
		},
		{
			name: "help short",
			args: []string{"-h"},
			check: func(f *Flags) error {
				if !f.Help {
					return errorf("Help should be true")
				}
				return nil
			},
		},
		{
			name: "help long",
			args: []string{"--help"},
			check: func(f *Flags) error {
				if !f.Help {
					return errorf("Help should be true")
				}
				return nil
			},
		},
		{
			name: "multiple flags",
			args: []string{"-m", "grid", "--sort", "project", "--asc", "-p", "test"},
			check: func(f *Flags) error {
				if f.Mode == nil || *f.Mode != "grid" {
					return errorf("Mode should be grid")
				}
				if f.SortField == nil || *f.SortField != "project" {
					return errorf("SortField should be project")
				}
				if f.SortAscending == nil || !*f.SortAscending {
					return errorf("SortAscending should be true")
				}
				if f.Project == nil || *f.Project != "test" {
					return errorf("Project should be test")
				}
				return nil
			},
		},
		{
			name:    "invalid flag",
			args:    []string{"--invalid-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := ParseFlags(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.check != nil {
				if err := tt.check(flags); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestFlagsApply(t *testing.T) {
	tests := []struct {
		name   string
		flags  *Flags
		check  func(*Config) error
	}{
		{
			name:  "nil flags",
			flags: nil,
			check: func(cfg *Config) error {
				// Config should be unchanged
				if cfg.Mode != DefaultMode {
					return errorf("Mode should be default")
				}
				return nil
			},
		},
		{
			name: "apply mode",
			flags: &Flags{
				Mode: strPtr("grid"),
			},
			check: func(cfg *Config) error {
				if cfg.Mode != DisplayModeGrid {
					return errorf("Mode should be grid")
				}
				return nil
			},
		},
		{
			name: "apply layout",
			flags: &Flags{
				Layout: strPtr("grid"),
			},
			check: func(cfg *Config) error {
				if cfg.Layout != LayoutTypeGrid {
					return errorf("Layout should be grid")
				}
				return nil
			},
		},
		{
			name: "apply columns",
			flags: &Flags{
				Columns: strPtr("name,status"),
			},
			check: func(cfg *Config) error {
				if len(cfg.Columns) != 2 {
					return errorf("should have 2 columns")
				}
				if cfg.Columns[0] != ColumnTypeName {
					return errorf("first column should be name")
				}
				if cfg.Columns[1] != ColumnTypeStatus {
					return errorf("second column should be status")
				}
				return nil
			},
		},
		{
			name: "apply refresh interval",
			flags: &Flags{
				RefreshInterval: durationPtr(15 * time.Second),
			},
			check: func(cfg *Config) error {
				if cfg.Refresh.Interval != 15*time.Second {
					return errorf("Refresh.Interval should be 15s")
				}
				return nil
			},
		},
		{
			name: "apply refresh enabled false",
			flags: &Flags{
				RefreshEnabled: boolPtr(false),
			},
			check: func(cfg *Config) error {
				if cfg.Refresh.Enabled != false {
					return errorf("Refresh.Enabled should be false")
				}
				return nil
			},
		},
		{
			name: "apply tmux enabled false",
			flags: &Flags{
				TmuxEnabled: boolPtr(false),
			},
			check: func(cfg *Config) error {
				if cfg.Tmux.Enabled != false {
					return errorf("Tmux.Enabled should be false")
				}
				return nil
			},
		},
		{
			name: "apply cache enabled false",
			flags: &Flags{
				CacheEnabled: boolPtr(false),
			},
			check: func(cfg *Config) error {
				if cfg.Cache.Enabled != false {
					return errorf("Cache.Enabled should be false")
				}
				return nil
			},
		},
		{
			name: "apply sort field",
			flags: &Flags{
				SortField: strPtr("project"),
			},
			check: func(cfg *Config) error {
				if cfg.Sort.Field != "project" {
					return errorf("Sort.Field should be project")
				}
				return nil
			},
		},
		{
			name: "apply sort ascending",
			flags: &Flags{
				SortAscending: boolPtr(true),
			},
			check: func(cfg *Config) error {
				if !cfg.Sort.Ascending {
					return errorf("Sort.Ascending should be true")
				}
				return nil
			},
		},
		{
			name: "apply project filter",
			flags: &Flags{
				Project: strPtr("myproject"),
			},
			check: func(cfg *Config) error {
				if cfg.Filter.Project != "myproject" {
					return errorf("Filter.Project should be myproject")
				}
				return nil
			},
		},
		{
			name: "apply max age filter",
			flags: &Flags{
				MaxAge: strPtr("7d"),
			},
			check: func(cfg *Config) error {
				if cfg.Filter.MaxAge != "7d" {
					return errorf("Filter.MaxAge should be 7d")
				}
				return nil
			},
		},
		{
			name: "apply exclude exited",
			flags: &Flags{
				ExcludeExited: boolPtr(true),
			},
			check: func(cfg *Config) error {
				if !cfg.Filter.ExcludeExited {
					return errorf("Filter.ExcludeExited should be true")
				}
				return nil
			},
		},
		{
			name: "apply multiple flags",
			flags: &Flags{
				Mode:          strPtr("grid"),
				SortField:     strPtr("project"),
				SortAscending: boolPtr(true),
				Project:       strPtr("test"),
			},
			check: func(cfg *Config) error {
				if cfg.Mode != DisplayModeGrid {
					return errorf("Mode should be grid")
				}
				if cfg.Sort.Field != "project" {
					return errorf("Sort.Field should be project")
				}
				if !cfg.Sort.Ascending {
					return errorf("Sort.Ascending should be true")
				}
				if cfg.Filter.Project != "test" {
					return errorf("Filter.Project should be test")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.flags.Apply(&cfg)
			if err := tt.check(&cfg); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestFlagsIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		flags Flags
		want  bool
	}{
		{
			name:  "empty flags",
			flags: Flags{},
			want:  true,
		},
		{
			name:  "config path only is still empty",
			flags: Flags{ConfigPath: "/path/to/config"},
			want:  true,
		},
		{
			name:  "mode set",
			flags: Flags{Mode: strPtr("grid")},
			want:  false,
		},
		{
			name:  "project set",
			flags: Flags{Project: strPtr("test")},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.flags.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlagsHasConfigPath(t *testing.T) {
	tests := []struct {
		name  string
		flags Flags
		want  bool
	}{
		{
			name:  "no config path",
			flags: Flags{},
			want:  false,
		},
		{
			name:  "has config path",
			flags: Flags{ConfigPath: "/path/to/config.yaml"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.flags.HasConfigPath(); got != tt.want {
				t.Errorf("HasConfigPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions for creating pointers to values
func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

// errorf returns an error with a formatted message
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func errorf(format string, args ...interface{}) error {
	return &testError{msg: sprintf(format, args...)}
}

func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	// Simple sprintf for test errors
	result := format
	for _, arg := range args {
		i := 0
		for i < len(result)-1 {
			if result[i] == '%' {
				switch result[i+1] {
				case 'v', 's', 'd', 'q':
					var replacement string
					switch v := arg.(type) {
					case string:
						if result[i+1] == 'q' {
							replacement = "\"" + v + "\""
						} else {
							replacement = v
						}
					case int:
						replacement = intToString(v)
					case bool:
						if v {
							replacement = "true"
						} else {
							replacement = "false"
						}
					default:
						replacement = "<value>"
					}
					result = result[:i] + replacement + result[i+2:]
					break
				}
			}
			i++
		}
	}
	return result
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
