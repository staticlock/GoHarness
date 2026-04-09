package ui

type InputSession struct {
	prompt    string
	vimMode   bool
	voiceMode bool
}

func NewInputSession() *InputSession {
	return &InputSession{
		prompt:    "> ",
		vimMode:   false,
		voiceMode: false,
	}
}

func (s *InputSession) SetModes(vimEnabled bool, voiceEnabled bool) {
	s.vimMode = vimEnabled
	s.voiceMode = voiceEnabled
	var parts []string
	if vimEnabled {
		parts = append(parts, "[vim]")
	}
	if voiceEnabled {
		parts = append(parts, "[voice]")
	}
	if len(parts) > 0 {
		s.prompt = join(parts) + "> "
	} else {
		s.prompt = "> "
	}
}

func (s *InputSession) Prompt() string {
	return s.prompt
}

func (s *InputSession) Ask(question string) string {
	return "[question] " + question + "\n> "
}

func join(parts []string) string {
	result := ""
	for _, p := range parts {
		result += p
	}
	return result
}
