package domain

type ChatMessage struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type TokenUsage struct {
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	CachedPromptTokens  int
}

func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:       u.PromptTokens + other.PromptTokens,
		CompletionTokens:   u.CompletionTokens + other.CompletionTokens,
		TotalTokens:        u.TotalTokens + other.TotalTokens,
		CachedPromptTokens: u.CachedPromptTokens + other.CachedPromptTokens,
	}
}

type LLMResult struct {
	Message AssistantMessage
	Usage   TokenUsage
}

type AssistantMessage struct {
	Content   string
	ToolCalls []ToolCall
	Thinking  string
}

type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}
