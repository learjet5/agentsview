package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodexReasoningSummaryAsSeparateMessage(t *testing.T) {
	content := strings.Join([]string{
		`{"timestamp":"2026-11-04T00:00:00Z","type":"session_meta","payload":{"id":"reasoning-1","cwd":"/tmp/test"}}`,
		`{"timestamp":"2026-11-04T00:00:00.5Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"Follow the task rules."}]}}`,
		`{"timestamp":"2026-11-04T00:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Inspect the file"}]}}`,
		`{"timestamp":"2026-11-04T00:00:02Z","type":"response_item","payload":{"type":"reasoning","summary":[{"type":"summary_text","text":"First inspect."},{"type":"summary_text","text":"Then decide."}],"encrypted_content":"opaque"}}`,
		`{"timestamp":"2026-11-04T00:00:03Z","type":"response_item","payload":{"type":"function_call","name":"shell_command","call_id":"call-1","arguments":"{\"command\":\"ls\"}"}}`,
		`{"timestamp":"2026-11-04T00:00:04Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"file.txt"}}`,
		`{"timestamp":"2026-11-04T00:00:05Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I found the file."}]}}`,
	}, "\n") + "\n"
	_, messages := runCodexParserTest(t, "reasoning-summary.jsonl", content, false)
	require.Len(t, messages, 5)
	assert.Equal(t, RoleSystem, messages[0].Role)
	assert.Equal(t, "developer", messages[0].SourceType)
	assert.Equal(t, "Follow the task rules.", messages[0].Content)
	assert.Equal(t, RoleUser, messages[1].Role)
	summary := messages[2]
	assert.Equal(t, RoleAssistant, summary.Role)
	assert.Equal(t, "First inspect.\n\nThen decide.", summary.ThinkingText)
	assert.Equal(t, "[Thinking]\nFirst inspect.\n\nThen decide.\n[/Thinking]", summary.Content)
	assert.True(t, summary.HasThinking)
	assert.Equal(t, "reasoning", summary.SourceType)
	assert.Equal(t, "reasoning_summary", summary.SourceSubtype)
	assert.NotContains(t, summary.Content, "opaque")
	assert.Equal(t, "shell_command", messages[3].ToolCalls[0].ToolName)
	assert.Equal(t, "I found the file.", messages[4].Content)
}

func TestParseCodexEmptyReasoningSummaryIsSkipped(t *testing.T) {
	content := strings.Join([]string{
		`{"timestamp":"2026-11-04T00:00:00Z","type":"session_meta","payload":{"id":"reasoning-2","cwd":"/tmp/test"}}`,
		`{"timestamp":"2026-11-04T00:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}}`,
		`{"timestamp":"2026-11-04T00:00:02Z","type":"response_item","payload":{"type":"reasoning","summary":[],"encrypted_content":"opaque"}}`,
	}, "\n") + "\n"
	_, messages := runCodexParserTest(t, "empty-reasoning-summary.jsonl", content, false)
	require.Len(t, messages, 1)
	assert.Equal(t, RoleUser, messages[0].Role)
}

func TestParseCodexEscapeInstructionAndCompaction(t *testing.T) {
	content := strings.Join([]string{
		`{"timestamp":"2026-11-04T00:00:00Z","type":"session_meta","payload":{"id":"aet-1","cwd":"/aet/exploitgym_glm_v1"}}`,
		`{"timestamp":"2026-11-04T00:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Solve the task"}]}}`,
		`{"timestamp":"2026-11-04T00:00:02Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"Injected guidance"}],"aet_annotation":{"kind":"escape_instruction"}}}`,
		`{"timestamp":"2026-11-04T00:00:03Z","type":"compacted","payload":{"message":"Another language model started to solve this problem.\nProgress so far."}}`,
		`{"timestamp":"2026-11-04T00:00:04Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Continuing."}]}}`,
	}, "\n") + "\n"
	_, messages := runCodexParserTest(t, "aet-escape-compaction.jsonl", content, false)
	require.Len(t, messages, 4)

	instruction := messages[1]
	assert.Equal(t, RoleSystem, instruction.Role)
	assert.Equal(t, "developer", instruction.SourceType)
	assert.Equal(t, "escape_instruction", instruction.SourceSubtype)
	assert.Equal(t, "Injected guidance", instruction.Content)

	boundary := messages[2]
	assert.True(t, boundary.IsCompactBoundary)
	assert.True(t, boundary.IsSystem)
	assert.Equal(t, "compact_boundary", boundary.SourceSubtype)
	assert.Equal(t, "Another language model started to solve this problem.\nProgress so far.", boundary.Content)

	assert.Equal(t, "Continuing.", messages[3].Content)
}
