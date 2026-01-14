// Package session provides types and functions for working with
// Claude Code sessions, including parsing JSONL files and caching.
package session

import (
	"encoding/json"
)

// MessageType represents the type of message in a JSONL line.
// These correspond to the top-level "type" field in each JSON object.
type MessageType string

const (
	// MessageTypeSystem represents system events (init, compaction, errors).
	MessageTypeSystem MessageType = "system"

	// MessageTypeUser represents user input, tool results, and sub-agent results.
	MessageTypeUser MessageType = "user"

	// MessageTypeAssistant represents Claude's responses.
	MessageTypeAssistant MessageType = "assistant"

	// MessageTypeSummary represents session summary/title.
	MessageTypeSummary MessageType = "summary"

	// MessageTypeFileHistorySnapshot represents file backup state snapshots.
	MessageTypeFileHistorySnapshot MessageType = "file-history-snapshot"

	// MessageTypeQueueOperation represents queue management operations.
	MessageTypeQueueOperation MessageType = "queue-operation"

	// MessageTypeResult represents session completion/result.
	MessageTypeResult MessageType = "result"
)

// SystemSubtype represents subtypes of system messages.
type SystemSubtype string

const (
	// SystemSubtypeInit is recorded at the start of each session.
	SystemSubtypeInit SystemSubtype = "init"

	// SystemSubtypeCompactBoundary is recorded when context is compacted.
	SystemSubtypeCompactBoundary SystemSubtype = "compact_boundary"

	// SystemSubtypeAPIError is recorded when API calls fail.
	SystemSubtypeAPIError SystemSubtype = "api_error"

	// SystemSubtypeLocalCommand is recorded when user runs a slash command.
	SystemSubtypeLocalCommand SystemSubtype = "local_command"

	// SystemSubtypeTurnDuration records timing for conversation turns.
	SystemSubtypeTurnDuration SystemSubtype = "turn_duration"
)

// ResultSubtype represents subtypes of result messages.
type ResultSubtype string

const (
	// ResultSubtypeSuccess indicates successful session completion.
	ResultSubtypeSuccess ResultSubtype = "success"

	// ResultSubtypeError indicates session ended with error.
	ResultSubtypeError ResultSubtype = "error"

	// ResultSubtypeCancelled indicates session was cancelled.
	ResultSubtypeCancelled ResultSubtype = "cancelled"
)

// ContentBlockType represents types of content blocks in assistant messages.
type ContentBlockType string

const (
	// ContentBlockTypeText represents plain text content.
	ContentBlockTypeText ContentBlockType = "text"

	// ContentBlockTypeThinking represents extended thinking/reasoning.
	ContentBlockTypeThinking ContentBlockType = "thinking"

	// ContentBlockTypeToolUse represents a tool call request.
	ContentBlockTypeToolUse ContentBlockType = "tool_use"

	// ContentBlockTypeToolResult represents a tool execution result.
	ContentBlockTypeToolResult ContentBlockType = "tool_result"
)

// BaseLine contains the common fields present in most JSONL lines.
// Not all fields are present in all message types.
type BaseLine struct {
	Type       MessageType `json:"type"`
	UUID       string      `json:"uuid,omitempty"`
	ParentUUID *string     `json:"parentUuid"` // Can be null
	Timestamp  string      `json:"timestamp,omitempty"`
	SessionID  string      `json:"sessionId,omitempty"`
	Version    string      `json:"version,omitempty"`
	CWD        string      `json:"cwd,omitempty"`
	GitBranch  string      `json:"gitBranch,omitempty"`
	IsSidechain bool       `json:"isSidechain,omitempty"`
	UserType   string      `json:"userType,omitempty"`
}

// SystemLine represents a system message in the JSONL.
type SystemLine struct {
	BaseLine
	Subtype         SystemSubtype    `json:"subtype"`
	Model           string           `json:"model,omitempty"`
	ClaudeCodeVersion string         `json:"claude_code_version,omitempty"`
	Content         string           `json:"content,omitempty"`
	CompactMetadata *CompactMetadata `json:"compactMetadata,omitempty"`
	DurationMs      int64            `json:"duration_ms,omitempty"`
}

// CompactMetadata contains token counts before and after compaction.
type CompactMetadata struct {
	PreTokens  int `json:"preTokens"`
	PostTokens int `json:"postTokens,omitempty"`
}

// UserLine represents a user message in the JSONL.
type UserLine struct {
	BaseLine
	Message          UserMessage       `json:"message"`
	ThinkingMetadata *ThinkingMetadata `json:"thinkingMetadata,omitempty"`
	Todos            []interface{}     `json:"todos,omitempty"`
	ToolUseResult    *ToolUseResult    `json:"toolUseResult,omitempty"`
	Slug             string            `json:"slug,omitempty"`
	IsMeta           bool              `json:"isMeta,omitempty"`
	AgentID          string            `json:"agentId,omitempty"`
}

// UserMessage contains the role and content of a user message.
// Content can be a string or an array of content blocks.
type UserMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []ContentBlock
}

// GetTextContent attempts to extract text content from the user message.
// If the content is a string, it returns that string.
// If the content is an array, it concatenates all text blocks.
func (m *UserMessage) GetTextContent() string {
	// Try to unmarshal as string first
	var strContent string
	if err := json.Unmarshal(m.Content, &strContent); err == nil {
		return strContent
	}

	// Try to unmarshal as array of content blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(m.Content, &blocks); err == nil {
		var text string
		for _, block := range blocks {
			if block.Type == string(ContentBlockTypeText) && block.Text != "" {
				text += block.Text
			}
		}
		return text
	}

	return ""
}

// ThinkingMetadata contains extended thinking settings.
type ThinkingMetadata struct {
	Level    string   `json:"level,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	Triggers []string `json:"triggers,omitempty"`
}

// ToolUseResult contains structured result metadata from tool execution.
// The exact fields depend on the tool type.
type ToolUseResult struct {
	// Common fields
	Type     string `json:"type,omitempty"`
	FilePath string `json:"filePath,omitempty"`

	// File operations
	Content        string        `json:"content,omitempty"`
	StructuredPatch []interface{} `json:"structuredPatch,omitempty"`
	OriginalFile   *string       `json:"originalFile,omitempty"`

	// Bash operations
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
	IsImage     bool   `json:"isImage,omitempty"`

	// Sub-agent results
	AgentID      string        `json:"agentId,omitempty"`
	TotalTokens  int           `json:"totalTokens,omitempty"`
	AgentContent []interface{} `json:"agentContent,omitempty"` // Sub-agent content blocks
}

// AssistantLine represents an assistant message in the JSONL.
type AssistantLine struct {
	BaseLine
	RequestID string           `json:"requestId,omitempty"`
	Slug      string           `json:"slug,omitempty"`
	Message   AssistantMessage `json:"message"`
}

// AssistantMessage contains the API response from Claude.
type AssistantMessage struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Model        string         `json:"model"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        *TokenUsage    `json:"usage,omitempty"`
}

// ContentBlock represents a block of content in an assistant message.
type ContentBlock struct {
	Type string `json:"type"`

	// Text content
	Text string `json:"text,omitempty"`

	// Thinking content
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`

	// Tool use content
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Tool result content
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// TokenUsage contains token usage statistics.
type TokenUsage struct {
	InputTokens              int           `json:"input_tokens"`
	OutputTokens             int           `json:"output_tokens"`
	CacheCreationInputTokens int           `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int           `json:"cache_read_input_tokens,omitempty"`
	CacheCreation            *CacheCreation `json:"cache_creation,omitempty"`
	ServiceTier              string        `json:"service_tier,omitempty"`
}

// CacheCreation contains detailed cache tier breakdown.
type CacheCreation struct {
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens,omitempty"`
}

// SummaryLine represents a session summary message.
// Note: Summary messages often lack common fields like uuid, timestamp, sessionId.
type SummaryLine struct {
	Type     MessageType `json:"type"`
	Summary  string      `json:"summary"`
	LeafUUID string      `json:"leafUuid,omitempty"`
}

// FileHistorySnapshotLine represents a file backup state snapshot.
type FileHistorySnapshotLine struct {
	Type             MessageType               `json:"type"`
	MessageID        string                    `json:"messageId"`
	IsSnapshotUpdate bool                      `json:"isSnapshotUpdate"`
	Snapshot         FileHistorySnapshotDetail `json:"snapshot"`
}

// FileHistorySnapshotDetail contains the snapshot data.
type FileHistorySnapshotDetail struct {
	MessageID          string                       `json:"messageId"`
	Timestamp          string                       `json:"timestamp"`
	TrackedFileBackups map[string]TrackedFileBackup `json:"trackedFileBackups"`
}

// TrackedFileBackup contains backup information for a single file.
type TrackedFileBackup struct {
	BackupFileName *string `json:"backupFileName"`
	Version        int     `json:"version"`
	BackupTime     string  `json:"backupTime"`
}

// QueueOperationLine represents a prompt queue operation.
type QueueOperationLine struct {
	BaseLine
	Operation string `json:"operation"`
	Content   string `json:"content,omitempty"`
}

// ResultLine represents session completion/result.
type ResultLine struct {
	BaseLine
	Subtype      ResultSubtype `json:"subtype"`
	TotalCostUSD float64       `json:"total_cost_usd"`
	DurationMs   int64         `json:"duration_ms"`
	NumTurns     int           `json:"num_turns"`
	Usage        *ResultUsage  `json:"usage,omitempty"`
}

// ResultUsage contains aggregate token usage for the session.
type ResultUsage struct {
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens,omitempty"`
}

// GenericLine is used for initial type detection when parsing JSONL.
// It only contains the type field to determine which specific type to unmarshal into.
type GenericLine struct {
	Type    MessageType   `json:"type"`
	Subtype string        `json:"subtype,omitempty"`
}

// ParseLine parses a raw JSON line and returns the appropriate typed struct.
// Returns the parsed struct and any error encountered.
func ParseLine(data []byte) (interface{}, error) {
	// First, detect the type
	var generic GenericLine
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, err
	}

	// Now parse into the specific type
	switch generic.Type {
	case MessageTypeSystem:
		var line SystemLine
		if err := json.Unmarshal(data, &line); err != nil {
			return nil, err
		}
		return &line, nil

	case MessageTypeUser:
		var line UserLine
		if err := json.Unmarshal(data, &line); err != nil {
			return nil, err
		}
		return &line, nil

	case MessageTypeAssistant:
		var line AssistantLine
		if err := json.Unmarshal(data, &line); err != nil {
			return nil, err
		}
		return &line, nil

	case MessageTypeSummary:
		var line SummaryLine
		if err := json.Unmarshal(data, &line); err != nil {
			return nil, err
		}
		return &line, nil

	case MessageTypeFileHistorySnapshot:
		var line FileHistorySnapshotLine
		if err := json.Unmarshal(data, &line); err != nil {
			return nil, err
		}
		return &line, nil

	case MessageTypeQueueOperation:
		var line QueueOperationLine
		if err := json.Unmarshal(data, &line); err != nil {
			return nil, err
		}
		return &line, nil

	case MessageTypeResult:
		var line ResultLine
		if err := json.Unmarshal(data, &line); err != nil {
			return nil, err
		}
		return &line, nil

	default:
		// Return the generic line for unknown types
		return &generic, nil
	}
}
