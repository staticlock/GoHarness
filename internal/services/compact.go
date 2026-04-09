package services

import (
	"strings"

	"github.com/staticlock/GoHarness/internal/engine"
)

const MaxSummaryLength = 300

func SummarizeMessages(messages []engine.ConversationMessage, maxMessages int) string {
	if len(messages) == 0 {
		return ""
	}

	start := 0
	if len(messages) > maxMessages {
		start = len(messages) - maxMessages
	}

	var lines []string
	for i := start; i < len(messages); i++ {
		msg := messages[i]
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			continue
		}
		if len(text) > MaxSummaryLength {
			text = text[:MaxSummaryLength]
		}
		lines = append(lines, msg.Role+": "+text)
	}
	return strings.Join(lines, "\n")
}

func CompactMessages(messages []engine.ConversationMessage, preserveRecent int) []engine.ConversationMessage {
	if len(messages) <= preserveRecent {
		return messages
	}

	older := messages[:len(messages)-preserveRecent]
	newer := messages[len(messages)-preserveRecent:]

	summary := SummarizeMessages(older, len(older))
	if summary == "" {
		return newer
	}

	summaryMsg := engine.ConversationMessage{
		Role: "assistant",
		Text: "[conversation summary]\n" + summary,
	}

	result := []engine.ConversationMessage{summaryMsg}
	result = append(result, newer...)
	return result
}

func EstimateConversationTokens(messages []engine.ConversationMessage) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg.Text)
	}
	return total
}
