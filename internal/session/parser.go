package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shitchell/claude-dashboard/internal/constants"
	"github.com/shitchell/claude-dashboard/internal/logging"
	"github.com/shitchell/claude-dashboard/internal/util"
)

// Parser error types for specific error conditions.
var (
	// ErrEmptyFile indicates the JSONL file is empty.
	ErrEmptyFile = errors.New("empty file")

	// ErrNoInitMessage indicates no system init message was found.
	ErrNoInitMessage = errors.New("no init message found")

	// ErrMalformedJSON indicates a JSON parsing error.
	ErrMalformedJSON = errors.New("malformed JSON")
)

// LastEntry contains information from the last line of a JSONL file.
// This is useful for quick state detection without parsing the entire file.
type LastEntry struct {
	// Type is the message type of the last entry.
	Type MessageType

	// Subtype is the subtype (for system and result messages).
	Subtype string

	// Timestamp is the timestamp of the last entry.
	Timestamp time.Time

	// Preview is a text preview from the last entry (if applicable).
	Preview string

	// IsToolResult is true if the last message is a tool result.
	IsToolResult bool

	// IsComplete is true if the session has a result message.
	IsComplete bool
}

// ParserConfig contains configuration options for the parser.
type ParserConfig struct {
	// MaxScanBytes is the maximum bytes to read when scanning from end.
	// Defaults to 64KB if not set.
	MaxScanBytes int64

	// BufferSize is the bufio.Scanner buffer size.
	// Defaults to 64KB if not set.
	BufferSize int
}

// DefaultParserConfig returns the default parser configuration.
func DefaultParserConfig() ParserConfig {
	return ParserConfig{
		MaxScanBytes: 64 * 1024, // 64KB
		BufferSize:   64 * 1024, // 64KB
	}
}

// Parser extracts metadata from JSONL session files.
type Parser struct {
	config ParserConfig
}

// NewParser creates a new Parser with the given configuration.
// If config is nil, default configuration is used.
func NewParser(config *ParserConfig) *Parser {
	if config == nil {
		defaultConfig := DefaultParserConfig()
		config = &defaultConfig
	}
	return &Parser{config: *config}
}

// Parse extracts metadata from a JSONL file without loading the entire file.
// It reads the file line by line to find session metadata from user/assistant
// messages and summary, counting messages and turns along the way.
func (p *Parser) Parse(path string) (*SessionMetadata, error) {
	logging.Debug("Starting to parse session file: %s", path)

	file, err := os.Open(path)
	if err != nil {
		logging.Debug("Failed to open session file: %s: %v", path, err)
		return nil, err
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		logging.Debug("Failed to stat session file: %s: %v", path, err)
		return nil, err
	}

	logging.Debug("Session file size: %s (%d bytes)", path, fileInfo.Size())

	if fileInfo.Size() == 0 {
		logging.Debug("Session file is empty: %s", path)
		return nil, ErrEmptyFile
	}

	// Extract session ID from filename
	fileName := filepath.Base(path)
	sessionID := strings.TrimSuffix(fileName, ".jsonl")
	if strings.HasPrefix(sessionID, "agent-") {
		sessionID = strings.TrimPrefix(sessionID, "agent-")
	}

	// Extract project info from directory structure
	projectDir := filepath.Base(filepath.Dir(path))
	projectPath, err := util.DecodeProjectPath(projectDir)
	if err != nil {
		// If we can't decode, use the raw directory name
		projectPath = projectDir
	}
	projectName := util.ProjectNameFromPath(projectPath)

	metadata := &SessionMetadata{
		ID:          sessionID,
		FilePath:    path,
		ProjectPath: projectPath,
		ProjectName: projectName,
		ModTime:     fileInfo.ModTime(),
	}

	// Create a scanner with a custom buffer size for large lines
	scanner := bufio.NewScanner(file)
	buf := make([]byte, p.config.BufferSize)
	scanner.Buffer(buf, p.config.BufferSize)

	var (
		foundMetadata bool // True once we've found CWD and Model
		messageCount  int
		turnCount     int
		lastUserUUID  string
		lastAssistant *AssistantLine
		lastPreview   string
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		messageCount++

		// Parse the line to detect type
		var generic GenericLine
		if err := json.Unmarshal(line, &generic); err != nil {
			// Log and skip malformed lines but continue parsing
			logging.Debug("Skipping malformed JSON line in %s at line %d: %v", path, messageCount, err)
			continue
		}

		switch generic.Type {
		case MessageTypeSystem:
			if generic.Subtype == string(SystemSubtypeInit) && !foundMetadata {
				var sysLine SystemLine
				if err := json.Unmarshal(line, &sysLine); err == nil {
					if sysLine.Model != "" {
						metadata.Model = sysLine.Model
					}
					if sysLine.CWD != "" {
						metadata.CWD = sysLine.CWD
					}
					if metadata.Model != "" && metadata.CWD != "" {
						foundMetadata = true
					}
				}
			}

		case MessageTypeUser:
			var userLine UserLine
			if err := json.Unmarshal(line, &userLine); err == nil {
				// Extract CWD from user message if not found yet
				if metadata.CWD == "" && userLine.CWD != "" {
					metadata.CWD = userLine.CWD
				}
				// Track for turn counting - a turn is a user message followed by assistant
				if userLine.UUID != "" {
					lastUserUUID = userLine.UUID
				}
				// If this is a regular user message (not tool result), extract preview
				if userLine.ToolUseResult == nil && !userLine.IsMeta {
					text := userLine.Message.GetTextContent()
					if text != "" && !strings.HasPrefix(text, "<") {
						lastPreview = truncatePreview(text)
					}
				}
			}

		case MessageTypeAssistant:
			var assistLine AssistantLine
			if err := json.Unmarshal(line, &assistLine); err == nil {
				lastAssistant = &assistLine
				// Extract model from assistant message if not found yet
				if metadata.Model == "" && assistLine.Message.Model != "" {
					metadata.Model = assistLine.Message.Model
				}
				// Check if we have all metadata now
				if !foundMetadata && metadata.Model != "" && metadata.CWD != "" {
					foundMetadata = true
				}
				// Count a turn when we have a matching parent
				if lastUserUUID != "" && assistLine.ParentUUID != nil && *assistLine.ParentUUID == lastUserUUID {
					turnCount++
					lastUserUUID = "" // Reset for next turn
				}
				// Extract preview from assistant message
				for _, block := range assistLine.Message.Content {
					if block.Type == string(ContentBlockTypeText) && block.Text != "" {
						lastPreview = truncatePreview(block.Text)
						break
					}
				}
			}

		case MessageTypeSummary:
			var summaryLine SummaryLine
			if err := json.Unmarshal(line, &summaryLine); err == nil {
				metadata.Summary = summaryLine.Summary
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// Check if it's a buffer overflow error
		if err == bufio.ErrTooLong {
			// Try to continue with what we have
		} else {
			return nil, err
		}
	}

	// We need at least a CWD to consider this a valid session
	// (Model might be empty for sessions that haven't had an assistant response yet)
	if metadata.CWD == "" {
		logging.Warn("Session file missing CWD, skipping: %s", path)
		return nil, ErrNoInitMessage
	}

	// Log if model is missing (not an error, just informational)
	if metadata.Model == "" {
		logging.Debug("Session file has no model set (possibly new session): %s", path)
	}

	metadata.MessageCount = messageCount
	metadata.TurnCount = turnCount
	metadata.Preview = lastPreview

	logging.Debug("Parsed session %s: %d messages, %d turns, model=%s, cwd=%s",
		sessionID, messageCount, turnCount, metadata.Model, metadata.CWD)

	// If no summary was found but we have an assistant message, use that for summary
	if metadata.Summary == "" && lastAssistant != nil {
		for _, block := range lastAssistant.Message.Content {
			if block.Type == string(ContentBlockTypeText) && block.Text != "" {
				// Use the first sentence or first N characters as summary
				metadata.Summary = extractSummary(block.Text)
				break
			}
		}
	}

	logging.Info("Successfully parsed session: %s (project=%s)", sessionID, metadata.ProjectName)
	return metadata, nil
}

// ParseLastEntry reads only the last line(s) of a JSONL file to get the final state.
// This is useful for quick status checks without parsing the entire file.
func (p *Parser) ParseLastEntry(path string) (*LastEntry, error) {
	logging.Debug("Parsing last entry from: %s", path)

	file, err := os.Open(path)
	if err != nil {
		logging.Debug("Failed to open file for last entry: %s: %v", path, err)
		return nil, err
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		logging.Debug("Failed to stat file for last entry: %s: %v", path, err)
		return nil, err
	}

	if fileInfo.Size() == 0 {
		logging.Debug("File is empty, cannot parse last entry: %s", path)
		return nil, ErrEmptyFile
	}

	// Seek to the end and read backwards to find the last line
	lastLine, err := p.readLastLine(file, fileInfo.Size())
	if err != nil {
		return nil, err
	}

	if len(lastLine) == 0 {
		return nil, ErrEmptyFile
	}

	// Parse the last line
	var generic GenericLine
	if err := json.Unmarshal(lastLine, &generic); err != nil {
		logging.Debug("Malformed JSON in last line of %s: %v", path, err)
		return nil, ErrMalformedJSON
	}

	entry := &LastEntry{
		Type:    generic.Type,
		Subtype: generic.Subtype,
	}

	// Parse based on type to extract more info
	switch generic.Type {
	case MessageTypeSystem:
		var sysLine SystemLine
		if err := json.Unmarshal(lastLine, &sysLine); err == nil {
			if sysLine.Timestamp != "" {
				entry.Timestamp, _ = time.Parse(time.RFC3339, sysLine.Timestamp)
			}
		}

	case MessageTypeUser:
		var userLine UserLine
		if err := json.Unmarshal(lastLine, &userLine); err == nil {
			if userLine.Timestamp != "" {
				entry.Timestamp, _ = time.Parse(time.RFC3339, userLine.Timestamp)
			}
			entry.IsToolResult = userLine.ToolUseResult != nil
			if !entry.IsToolResult {
				entry.Preview = truncatePreview(userLine.Message.GetTextContent())
			}
		}

	case MessageTypeAssistant:
		var assistLine AssistantLine
		if err := json.Unmarshal(lastLine, &assistLine); err == nil {
			if assistLine.Timestamp != "" {
				entry.Timestamp, _ = time.Parse(time.RFC3339, assistLine.Timestamp)
			}
			for _, block := range assistLine.Message.Content {
				if block.Type == string(ContentBlockTypeText) && block.Text != "" {
					entry.Preview = truncatePreview(block.Text)
					break
				}
			}
		}

	case MessageTypeSummary:
		var summaryLine SummaryLine
		if err := json.Unmarshal(lastLine, &summaryLine); err == nil {
			entry.Preview = summaryLine.Summary
		}

	case MessageTypeResult:
		var resultLine ResultLine
		if err := json.Unmarshal(lastLine, &resultLine); err == nil {
			entry.IsComplete = true
			entry.Subtype = string(resultLine.Subtype)
			if resultLine.Timestamp != "" {
				entry.Timestamp, _ = time.Parse(time.RFC3339, resultLine.Timestamp)
			}
		}
	}

	logging.Debug("Last entry parsed: type=%s, subtype=%s, complete=%v", entry.Type, entry.Subtype, entry.IsComplete)
	return entry, nil
}

// readLastLine reads the last non-empty line from the file.
// It reads backwards from the end, up to MaxScanBytes.
func (p *Parser) readLastLine(file *os.File, size int64) ([]byte, error) {
	// Calculate how much to read from the end
	readSize := p.config.MaxScanBytes
	if size < readSize {
		readSize = size
	}

	// Seek to near the end
	offset := size - readSize
	if offset < 0 {
		offset = 0
	}
	_, err := file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	// Read the chunk
	chunk := make([]byte, readSize)
	n, err := io.ReadFull(file, chunk)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	chunk = chunk[:n]

	// Find the last complete line (ends with newline or EOF)
	lines := splitLines(chunk)
	if len(lines) == 0 {
		return nil, nil
	}

	// Return the last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(string(lines[i]))
		if line != "" {
			return []byte(line), nil
		}
	}

	return nil, nil
}

// splitLines splits a byte slice into lines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := data[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	// Don't forget the last line if it doesn't end with newline
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// truncatePreview truncates text to the maximum preview length.
func truncatePreview(text string) string {
	// Clean up the text first
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Collapse multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	if len(text) <= constants.MaxPreviewLength {
		return text
	}
	return text[:constants.MaxPreviewLength-3] + "..."
}

// extractSummary extracts a summary from text, using the first sentence
// or first N characters.
func extractSummary(text string) string {
	text = strings.TrimSpace(text)

	// Try to find the first sentence
	sentenceEnders := []string{". ", ".\n", "! ", "!\n", "? ", "?\n"}
	minIdx := len(text)
	for _, ender := range sentenceEnders {
		if idx := strings.Index(text, ender); idx > 0 && idx < minIdx {
			minIdx = idx + 1 // Include the punctuation
		}
	}

	if minIdx < len(text) && minIdx <= constants.MaxPreviewLength {
		return text[:minIdx]
	}

	// Fall back to truncation
	return truncatePreview(text)
}
