package ports

import (
	"context"

	"github.com/Moksh-10/nl-lf-tracing/internal/domain"
)

type LLM interface {
	Chat(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolDef) (domain.LLMResult, error)
	WarmCache(ctx context.Context, systemPrompt string) (domain.TokenUsage, error)
}

type Tracer interface {
	Workflow(ctx context.Context, name string, meta WorkflowMeta, input any, fn func(context.Context) error) error
	AgentTurn(ctx context.Context, turn int, fn func(context.Context) error) error
	LLMCall(ctx context.Context, name, model string, messages []domain.ChatMessage, meta map[string]any, fn func(context.Context) (domain.LLMResult, error)) (domain.LLMResult, error)
	ToolCall(ctx context.Context, toolName string, args map[string]any, fn func(context.Context) (string, error)) (string, error)
	Flush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type WorkflowMeta struct {
	SessionID string
	UserID    string
	Tags      map[string]string
}
