package services

func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	length := len(text)
	if length <= 4 {
		return 1
	}
	return (length + 3) / 4
}

func EstimateMessageTokens(messages []string) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg)
	}
	return total
}
