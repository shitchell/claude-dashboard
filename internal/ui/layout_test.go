package ui

import (
	"testing"
)

// TestLayoutTypeString verifies LayoutType.String() returns correct values.
func TestLayoutTypeString(t *testing.T) {
	tests := []struct {
		name     string
		layout   LayoutType
		expected string
	}{
		{"list", LayoutTypeList, "list"},
		{"grid", LayoutTypeGrid, "grid"},
		{"unknown", LayoutType(999), "list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.layout.String()
			if result != tt.expected {
				t.Errorf("LayoutType(%d).String() = %q, want %q", tt.layout, result, tt.expected)
			}
		})
	}
}

// TestLayoutTypeConstants verifies layout type constant values are distinct.
func TestLayoutTypeConstants(t *testing.T) {
	if LayoutTypeList == LayoutTypeGrid {
		t.Error("LayoutTypeList and LayoutTypeGrid should have different values")
	}
}
