package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/Moksh-10/nl-lf-tracing/internal/domain"
)

type Ollama struct {
	client    *openai.Client
	model     string
	keepAlive string
}

func NewOllama(baseURL, model, keepAlive string) *Ollama {
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = strings.TrimRight(baseURL, "/")
	return &Ollama{
		client:    openai.NewClientWithConfig(cfg),
		model:     model,
		keepAlive: keepAlive,
	}
}

func (o *Ollama) Chat(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolDef) (domain.LLMResult, error) {
	req := openai.ChatCompletionRequest{
		Model:       o.model,
		Messages:    toOpenAIMessages(messages),
		Temperature: 0.2,
	}
	if len(tools) > 0 {
		req.Tools = toOpenAITools(tools)
	}

	resp, err := o.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return domain.LLMResult{}, err
	}
	if len(resp.Choices) == 0 {
		return domain.LLMResult{}, fmt.Errorf("ollama: empty choices")
	}
	ch := resp.Choices[0].Message
	out := domain.AssistantMessage{Content: ch.Content}
	for _, tc := range ch.ToolCalls {
		var args map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		out.ToolCalls = append(out.ToolCalls, domain.ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: args,
		})
	}
	usage := domain.TokenUsage{}
	if resp.Usage.TotalTokens > 0 {
		usage = domain.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	return domain.LLMResult{Message: out, Usage: usage}, nil
}

func (o *Ollama) WarmCache(ctx context.Context, systemPrompt string) (domain.TokenUsage, error) {
	res, err := o.Chat(ctx, []domain.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "ack"},
	}, nil)
	if err != nil {
		return domain.TokenUsage{}, err
	}
	return res.Usage, nil
}

func toOpenAIMessages(in []domain.ChatMessage) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, 0, len(in))
	for _, m := range in {
		msg := openai.ChatCompletionMessage{Role: m.Role, Content: m.Content, Name: m.Name, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			args, _ := json.Marshal(tc.Arguments)
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID: tc.ID, Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: tc.Name, Arguments: string(args)},
			})
		}
		out = append(out, msg)
	}
	return out
}

func toOpenAITools(defs []domain.ToolDef) []openai.Tool {
	out := make([]openai.Tool, 0, len(defs))
	for _, d := range defs {
		out = append(out, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  d.Parameters,
			},
		})
	}
	return out
}
