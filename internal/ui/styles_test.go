package ui

import (
	"testing"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// TestNewStyles verifies that NewStyles creates a valid Styles struct.
func TestNewStyles(t *testing.T) {
	styles := NewStyles()

	// Verify that all styles are initialized (not zero values with no properties)
	// Check a sampling of styles to ensure they have expected properties

	// Title should be bold
	if styles.Title.GetBold() != true {
		t.Error("expected Title style to be bold")
	}

	// StatusActive should be bold
	if styles.StatusActive.GetBold() != true {
		t.Error("expected StatusActive style to be bold")
	}

	// ItemSelected should be bold
	if styles.ItemSelected.GetBold() != true {
		t.Error("expected ItemSelected style to be bold")
	}

	// Error should be bold
	if styles.Error.GetBold() != true {
		t.Error("expected Error style to be bold")
	}
}

// TestNewSidePanelStyles verifies side panel styles are more compact.
func TestNewSidePanelStyles(t *testing.T) {
	regularStyles := NewStyles()
	sidePanelStyles := NewSidePanelStyles()

	// Side panel styles should have no padding on items
	regularPadding := regularStyles.ItemNormal.GetHorizontalPadding()
	sidePanelPadding := sidePanelStyles.ItemNormal.GetHorizontalPadding()

	if regularPadding <= sidePanelPadding {
		t.Errorf("expected side panel ItemNormal padding (%d) to be less than regular (%d)",
			sidePanelPadding, regularPadding)
	}
}

// TestStatusStyle verifies StatusStyle returns correct style for each status.
func TestStatusStyle(t *testing.T) {
	styles := NewStyles()

	tests := []struct {
		status       session.Status
		expectedBold bool
	}{
		{session.StatusActive, true},  // Active should be bold
		{session.StatusIdle, false},   // Idle should not be bold
		{session.StatusExited, false}, // Exited should not be bold
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			style := styles.StatusStyle(tt.status)
			if style.GetBold() != tt.expectedBold {
				t.Errorf("StatusStyle(%v).GetBold() = %v, want %v",
					tt.status, style.GetBold(), tt.expectedBold)
			}
		})
	}
}

// TestStatusIndicator verifies indicator characters for each status.
func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		status   session.Status
		expected string
	}{
		{session.StatusActive, IndicatorActive},
		{session.StatusIdle, IndicatorIdle},
		{session.StatusExited, IndicatorExited},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			indicator := StatusIndicator(tt.status)
			if indicator != tt.expected {
				t.Errorf("StatusIndicator(%v) = %q, want %q",
					tt.status, indicator, tt.expected)
			}
		})
	}
}

// TestRenderStatusIndicator verifies that RenderStatusIndicator produces output.
func TestRenderStatusIndicator(t *testing.T) {
	styles := NewStyles()

	tests := []session.Status{
		session.StatusActive,
		session.StatusIdle,
		session.StatusExited,
	}

	for _, status := range tests {
		t.Run(status.String(), func(t *testing.T) {
			rendered := styles.RenderStatusIndicator(status)

			// Rendered output should contain the indicator character
			indicator := StatusIndicator(status)
			if len(rendered) == 0 {
				t.Error("expected non-empty rendered indicator")
			}
			// The rendered output may include ANSI escape codes, but should contain the indicator
			if len(indicator) > 0 && len(rendered) < len(indicator) {
				t.Errorf("rendered indicator %q shorter than raw indicator %q",
					rendered, indicator)
			}
		})
	}
}

// TestDefaultStyles verifies that the package-level DefaultStyles is initialized.
func TestDefaultStyles(t *testing.T) {
	// DefaultStyles should be initialized
	if DefaultStyles.Title.GetBold() != true {
		t.Error("expected DefaultStyles.Title to be bold")
	}
}

// TestWithWidth verifies WithWidth creates a style with specified width.
func TestWithWidth(t *testing.T) {
	style := NewStyles().Name
	withWidth := WithWidth(style, 20)

	if withWidth.GetWidth() != 20 {
		t.Errorf("WithWidth() width = %d, want 20", withWidth.GetWidth())
	}
}

// TestWithMaxWidth verifies WithMaxWidth creates a style with specified max width.
func TestWithMaxWidth(t *testing.T) {
	style := NewStyles().Preview
	withMaxWidth := WithMaxWidth(style, 50)

	if withMaxWidth.GetMaxWidth() != 50 {
		t.Errorf("WithMaxWidth() maxWidth = %d, want 50", withMaxWidth.GetMaxWidth())
	}
}

// TestColorConstants verifies color constants are properly defined.
func TestColorConstants(t *testing.T) {
	// Verify that color constants are non-empty strings
	colors := []struct {
		name  string
		color string
	}{
		{"ColorPrimary", string(ColorPrimary)},
		{"ColorSecondary", string(ColorSecondary)},
		{"ColorWhite", string(ColorWhite)},
		{"ColorGray", string(ColorGray)},
		{"ColorActive", string(ColorActive)},
		{"ColorIdle", string(ColorIdle)},
		{"ColorExited", string(ColorExited)},
		{"ColorError", string(ColorError)},
	}

	for _, c := range colors {
		t.Run(c.name, func(t *testing.T) {
			if c.color == "" {
				t.Errorf("%s is empty", c.name)
			}
		})
	}
}

// TestIndicatorConstants verifies indicator constants are properly defined.
func TestIndicatorConstants(t *testing.T) {
	// Active indicator should be visible
	if len(IndicatorActive) == 0 {
		t.Error("IndicatorActive is empty")
	}

	// Idle indicator should be visible
	if len(IndicatorIdle) == 0 {
		t.Error("IndicatorIdle is empty")
	}

	// Exited indicator can be a space (invisible), but should have length
	if len(IndicatorExited) == 0 {
		t.Error("IndicatorExited is empty")
	}
}
