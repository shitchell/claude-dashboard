package session

import (
	"encoding/json"
	"testing"
)

func TestParseLineSystem(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"init","uuid":"test-uuid","timestamp":"2025-12-27T23:21:01.457Z","sessionId":"session-id","model":"claude-opus-4-5-20251101","claude_code_version":"2.0.76","cwd":"/home/user/project"}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	sysLine, ok := result.(*SystemLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *SystemLine", result)
	}

	if sysLine.Type != MessageTypeSystem {
		t.Errorf("Type = %q, want %q", sysLine.Type, MessageTypeSystem)
	}

	if sysLine.Subtype != SystemSubtypeInit {
		t.Errorf("Subtype = %q, want %q", sysLine.Subtype, SystemSubtypeInit)
	}

	if sysLine.Model != "claude-opus-4-5-20251101" {
		t.Errorf("Model = %q, want %q", sysLine.Model, "claude-opus-4-5-20251101")
	}

	if sysLine.CWD != "/home/user/project" {
		t.Errorf("CWD = %q, want %q", sysLine.CWD, "/home/user/project")
	}
}

func TestParseLineUser(t *testing.T) {
	data := []byte(`{"type":"user","uuid":"user-uuid","parentUuid":null,"timestamp":"2025-12-27T23:21:01.457Z","sessionId":"session-id","message":{"role":"user","content":"Hello world"}}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	userLine, ok := result.(*UserLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *UserLine", result)
	}

	if userLine.Type != MessageTypeUser {
		t.Errorf("Type = %q, want %q", userLine.Type, MessageTypeUser)
	}

	if userLine.Message.Role != "user" {
		t.Errorf("Message.Role = %q, want %q", userLine.Message.Role, "user")
	}

	content := userLine.Message.GetTextContent()
	if content != "Hello world" {
		t.Errorf("GetTextContent() = %q, want %q", content, "Hello world")
	}
}

func TestParseLineUserWithContentBlocks(t *testing.T) {
	data := []byte(`{"type":"user","uuid":"user-uuid","message":{"role":"user","content":[{"type":"text","text":"Block 1"},{"type":"text","text":" Block 2"}]}}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	userLine, ok := result.(*UserLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *UserLine", result)
	}

	content := userLine.Message.GetTextContent()
	if content != "Block 1 Block 2" {
		t.Errorf("GetTextContent() = %q, want %q", content, "Block 1 Block 2")
	}
}

func TestParseLineAssistant(t *testing.T) {
	data := []byte(`{"type":"assistant","uuid":"assist-uuid","parentUuid":"user-uuid","timestamp":"2025-12-27T23:21:07.039Z","requestId":"req_123","message":{"id":"msg_123","type":"message","model":"claude-opus-4-5-20251101","role":"assistant","content":[{"type":"text","text":"I can help with that."}],"stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":50}}}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	assistLine, ok := result.(*AssistantLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *AssistantLine", result)
	}

	if assistLine.Type != MessageTypeAssistant {
		t.Errorf("Type = %q, want %q", assistLine.Type, MessageTypeAssistant)
	}

	if assistLine.Message.Model != "claude-opus-4-5-20251101" {
		t.Errorf("Message.Model = %q, want %q", assistLine.Message.Model, "claude-opus-4-5-20251101")
	}

	if len(assistLine.Message.Content) != 1 {
		t.Fatalf("len(Message.Content) = %d, want 1", len(assistLine.Message.Content))
	}

	if assistLine.Message.Content[0].Type != "text" {
		t.Errorf("Content[0].Type = %q, want %q", assistLine.Message.Content[0].Type, "text")
	}

	if assistLine.Message.Content[0].Text != "I can help with that." {
		t.Errorf("Content[0].Text = %q, want %q", assistLine.Message.Content[0].Text, "I can help with that.")
	}

	if assistLine.Message.Usage == nil {
		t.Fatal("Message.Usage is nil")
	}

	if assistLine.Message.Usage.InputTokens != 100 {
		t.Errorf("Usage.InputTokens = %d, want 100", assistLine.Message.Usage.InputTokens)
	}
}

func TestParseLineSummary(t *testing.T) {
	data := []byte(`{"type":"summary","summary":"Session about Python development","leafUuid":"leaf-uuid"}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	summaryLine, ok := result.(*SummaryLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *SummaryLine", result)
	}

	if summaryLine.Type != MessageTypeSummary {
		t.Errorf("Type = %q, want %q", summaryLine.Type, MessageTypeSummary)
	}

	if summaryLine.Summary != "Session about Python development" {
		t.Errorf("Summary = %q, want %q", summaryLine.Summary, "Session about Python development")
	}

	if summaryLine.LeafUUID != "leaf-uuid" {
		t.Errorf("LeafUUID = %q, want %q", summaryLine.LeafUUID, "leaf-uuid")
	}
}

func TestParseLineResult(t *testing.T) {
	data := []byte(`{"type":"result","subtype":"success","uuid":"result-uuid","timestamp":"2025-12-27T23:50:00.000Z","sessionId":"session-id","total_cost_usd":0.1234,"duration_ms":180000,"num_turns":15,"usage":{"input_tokens":50000,"output_tokens":10000}}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	resultLine, ok := result.(*ResultLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *ResultLine", result)
	}

	if resultLine.Type != MessageTypeResult {
		t.Errorf("Type = %q, want %q", resultLine.Type, MessageTypeResult)
	}

	if resultLine.Subtype != ResultSubtypeSuccess {
		t.Errorf("Subtype = %q, want %q", resultLine.Subtype, ResultSubtypeSuccess)
	}

	if resultLine.TotalCostUSD != 0.1234 {
		t.Errorf("TotalCostUSD = %f, want 0.1234", resultLine.TotalCostUSD)
	}

	if resultLine.DurationMs != 180000 {
		t.Errorf("DurationMs = %d, want 180000", resultLine.DurationMs)
	}

	if resultLine.NumTurns != 15 {
		t.Errorf("NumTurns = %d, want 15", resultLine.NumTurns)
	}
}

func TestParseLineFileHistorySnapshot(t *testing.T) {
	data := []byte(`{"type":"file-history-snapshot","messageId":"msg-id","isSnapshotUpdate":false,"snapshot":{"messageId":"msg-id","timestamp":"2025-12-27T23:21:01.467Z","trackedFileBackups":{}}}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	snapshot, ok := result.(*FileHistorySnapshotLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *FileHistorySnapshotLine", result)
	}

	if snapshot.Type != MessageTypeFileHistorySnapshot {
		t.Errorf("Type = %q, want %q", snapshot.Type, MessageTypeFileHistorySnapshot)
	}

	if snapshot.MessageID != "msg-id" {
		t.Errorf("MessageID = %q, want %q", snapshot.MessageID, "msg-id")
	}

	if snapshot.IsSnapshotUpdate {
		t.Error("IsSnapshotUpdate = true, want false")
	}
}

func TestParseLineQueueOperation(t *testing.T) {
	data := []byte(`{"type":"queue-operation","uuid":"queue-uuid","timestamp":"2025-12-27T23:30:00.000Z","sessionId":"session-id","operation":"add","content":"Please also add tests"}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	queueLine, ok := result.(*QueueOperationLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *QueueOperationLine", result)
	}

	if queueLine.Type != MessageTypeQueueOperation {
		t.Errorf("Type = %q, want %q", queueLine.Type, MessageTypeQueueOperation)
	}

	if queueLine.Operation != "add" {
		t.Errorf("Operation = %q, want %q", queueLine.Operation, "add")
	}

	if queueLine.Content != "Please also add tests" {
		t.Errorf("Content = %q, want %q", queueLine.Content, "Please also add tests")
	}
}

func TestParseLineUnknownType(t *testing.T) {
	data := []byte(`{"type":"unknown-type","foo":"bar"}`)

	result, err := ParseLine(data)
	if err != nil {
		t.Fatalf("ParseLine() error = %v", err)
	}

	generic, ok := result.(*GenericLine)
	if !ok {
		t.Fatalf("ParseLine() returned %T, want *GenericLine", result)
	}

	if generic.Type != "unknown-type" {
		t.Errorf("Type = %q, want %q", generic.Type, "unknown-type")
	}
}

func TestParseLineMalformedJSON(t *testing.T) {
	data := []byte(`{this is not valid json}`)

	_, err := ParseLine(data)
	if err == nil {
		t.Error("ParseLine() should return error for malformed JSON")
	}
}

func TestUserMessageGetTextContentEmpty(t *testing.T) {
	msg := UserMessage{
		Role:    "user",
		Content: json.RawMessage(`[]`),
	}

	content := msg.GetTextContent()
	if content != "" {
		t.Errorf("GetTextContent() = %q, want empty string", content)
	}
}

func TestMessageTypeConstants(t *testing.T) {
	// Ensure message type constants have expected values
	tests := []struct {
		constant MessageType
		expected string
	}{
		{MessageTypeSystem, "system"},
		{MessageTypeUser, "user"},
		{MessageTypeAssistant, "assistant"},
		{MessageTypeSummary, "summary"},
		{MessageTypeFileHistorySnapshot, "file-history-snapshot"},
		{MessageTypeQueueOperation, "queue-operation"},
		{MessageTypeResult, "result"},
	}

	for _, tt := range tests {
		if string(tt.constant) != tt.expected {
			t.Errorf("MessageType constant = %q, want %q", tt.constant, tt.expected)
		}
	}
}

func TestSystemSubtypeConstants(t *testing.T) {
	tests := []struct {
		constant SystemSubtype
		expected string
	}{
		{SystemSubtypeInit, "init"},
		{SystemSubtypeCompactBoundary, "compact_boundary"},
		{SystemSubtypeAPIError, "api_error"},
		{SystemSubtypeLocalCommand, "local_command"},
		{SystemSubtypeTurnDuration, "turn_duration"},
	}

	for _, tt := range tests {
		if string(tt.constant) != tt.expected {
			t.Errorf("SystemSubtype constant = %q, want %q", tt.constant, tt.expected)
		}
	}
}

func TestResultSubtypeConstants(t *testing.T) {
	tests := []struct {
		constant ResultSubtype
		expected string
	}{
		{ResultSubtypeSuccess, "success"},
		{ResultSubtypeError, "error"},
		{ResultSubtypeCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if string(tt.constant) != tt.expected {
			t.Errorf("ResultSubtype constant = %q, want %q", tt.constant, tt.expected)
		}
	}
}

func TestContentBlockTypeConstants(t *testing.T) {
	tests := []struct {
		constant ContentBlockType
		expected string
	}{
		{ContentBlockTypeText, "text"},
		{ContentBlockTypeThinking, "thinking"},
		{ContentBlockTypeToolUse, "tool_use"},
		{ContentBlockTypeToolResult, "tool_result"},
	}

	for _, tt := range tests {
		if string(tt.constant) != tt.expected {
			t.Errorf("ContentBlockType constant = %q, want %q", tt.constant, tt.expected)
		}
	}
}
