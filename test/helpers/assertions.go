package helpers

import (
	"reflect"
	"strings"
	"testing"

	"github.com/shitchell/claude-dashboard/internal/session"
)

// AssertSessionsEqual compares two sessions and reports differences.
func AssertSessionsEqual(t *testing.T, got, want *session.Session) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil {
		t.Errorf("Got nil session, want %+v", want)
		return
	}
	if want == nil {
		t.Errorf("Got %+v, want nil session", got)
		return
	}

	if got.ID != want.ID {
		t.Errorf("Session.ID = %q, want %q", got.ID, want.ID)
	}
	if got.Model != want.Model {
		t.Errorf("Session.Model = %q, want %q", got.Model, want.Model)
	}
	if got.ProjectName != want.ProjectName {
		t.Errorf("Session.ProjectName = %q, want %q", got.ProjectName, want.ProjectName)
	}
	if got.ProjectPath != want.ProjectPath {
		t.Errorf("Session.ProjectPath = %q, want %q", got.ProjectPath, want.ProjectPath)
	}
	if got.Summary != want.Summary {
		t.Errorf("Session.Summary = %q, want %q", got.Summary, want.Summary)
	}
	if got.TmuxPane != want.TmuxPane {
		t.Errorf("Session.TmuxPane = %q, want %q", got.TmuxPane, want.TmuxPane)
	}
	if got.MessageCount != want.MessageCount {
		t.Errorf("Session.MessageCount = %d, want %d", got.MessageCount, want.MessageCount)
	}
	if got.TurnCount != want.TurnCount {
		t.Errorf("Session.TurnCount = %d, want %d", got.TurnCount, want.TurnCount)
	}
}

// AssertContains checks if a string contains a substring.
func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("Expected %q to contain %q", truncate(haystack, 100), needle)
	}
}

// AssertNotContains checks if a string does not contain a substring.
func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("Expected %q to NOT contain %q", truncate(haystack, 100), needle)
	}
}

// AssertEqual checks if two values are equal using reflect.DeepEqual.
func AssertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
}

// AssertNotEqual checks if two values are not equal.
func AssertNotEqual(t *testing.T, got, notWant interface{}) {
	t.Helper()
	if reflect.DeepEqual(got, notWant) {
		t.Errorf("Got %v, did not want %v", got, notWant)
	}
}

// AssertNil checks if a value is nil.
func AssertNil(t *testing.T, got interface{}) {
	t.Helper()
	if got != nil {
		// Handle interface nil check properly
		v := reflect.ValueOf(got)
		if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
			if !v.IsNil() {
				t.Errorf("Expected nil, got %v", got)
			}
		} else {
			t.Errorf("Expected nil, got %v", got)
		}
	}
}

// AssertNotNil checks if a value is not nil.
func AssertNotNil(t *testing.T, got interface{}) {
	t.Helper()
	if got == nil {
		t.Error("Expected non-nil value, got nil")
		return
	}
	v := reflect.ValueOf(got)
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			t.Error("Expected non-nil value, got nil")
		}
	}
}

// AssertNoError checks that an error is nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// AssertError checks that an error is not nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}

// AssertErrorContains checks that an error message contains a substring.
func AssertErrorContains(t *testing.T, err error, substring string) {
	t.Helper()
	if err == nil {
		t.Errorf("Expected error containing %q, got nil", substring)
		return
	}
	if !strings.Contains(err.Error(), substring) {
		t.Errorf("Expected error to contain %q, got %q", substring, err.Error())
	}
}

// AssertLen checks that a slice or map has the expected length.
func AssertLen(t *testing.T, collection interface{}, expectedLen int) {
	t.Helper()
	v := reflect.ValueOf(collection)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Map && v.Kind() != reflect.Array {
		t.Fatalf("AssertLen requires a slice, array, or map, got %T", collection)
	}
	if v.Len() != expectedLen {
		t.Errorf("Expected length %d, got %d", expectedLen, v.Len())
	}
}

// AssertTrue checks that a boolean is true.
func AssertTrue(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	if !value {
		if len(msgAndArgs) > 0 {
			t.Errorf("Expected true: %v", msgAndArgs)
		} else {
			t.Error("Expected true, got false")
		}
	}
}

// AssertFalse checks that a boolean is false.
func AssertFalse(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	if value {
		if len(msgAndArgs) > 0 {
			t.Errorf("Expected false: %v", msgAndArgs)
		} else {
			t.Error("Expected false, got true")
		}
	}
}

// AssertSliceContains checks if a slice contains a specific element.
func AssertSliceContains[T comparable](t *testing.T, slice []T, element T) {
	t.Helper()
	for _, item := range slice {
		if item == element {
			return
		}
	}
	t.Errorf("Expected slice to contain %v, but it doesn't", element)
}

// AssertStringSliceContains checks if a string slice contains an element.
func AssertStringSliceContains(t *testing.T, slice []string, element string) {
	t.Helper()
	AssertSliceContains(t, slice, element)
}

// truncate shortens a string for display in error messages.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
