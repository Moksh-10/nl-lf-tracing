package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Moksh-10/nl-lf-tracing/internal/domain"
	"github.com/Moksh-10/nl-lf-tracing/internal/ports"
)

const systemPrompt = `You are a local coding agent on the user's machine (Ollama).
Use tools to read/write/search files, run shell commands, and Tavily for web search/fetch when needed.
Prefer small focused tool calls. Use TodoWrite for multi-step work.`

type Agent struct {
	LLM      ports.LLM
	Tracer   ports.Tracer
	Tools    ToolExecutor
	Model    string
	MaxTurns int
	Warm     bool
}

type ToolExecutor interface {
	Dispatch(ctx context.Context, name string, args map[string]any) (string, error)
}

type RunOpts struct {
	SessionID    string
	UserID       string
	WorkflowName string
	Depth        int
}

func (a *Agent) Run(ctx context.Context, userMessage string, opts RunOpts) (string, error) {
	if opts.WorkflowName == "" {
		opts.WorkflowName = "nl-lf-agent-chat"
	}
	if opts.Depth == 0 && a.Warm {
		if warmer, ok := a.LLM.(interface {
			WarmCache(context.Context, string) (domain.TokenUsage, error)
		}); ok {
			if usage, err := warmer.WarmCache(ctx, systemPrompt); err == nil {
				slog.Info("prompt.prefix_warmed",
					"prompt_tokens", usage.PromptTokens,
					"completion_tokens", usage.CompletionTokens,
				)
			}
		}
	}

	cacheMeta := map[string]any{"system_prompt_chars": len(systemPrompt)}
	var convo []domain.ChatMessage
	convo = append(convo, domain.ChatMessage{Role: "user", Content: userMessage})
	tools := domain.ToolDefinitions()

	var total domain.TokenUsage
	final := ""

	meta := ports.WorkflowMeta{
		SessionID: opts.SessionID,
		UserID:    opts.UserID,
		Tags:      map[string]string{"compare": "langfuse+neatlogs"},
	}

	err := a.Tracer.Workflow(ctx, opts.WorkflowName, meta, map[string]any{
		"message": userMessage,
		"depth":   opts.Depth,
	}, func(ctx context.Context) error {
		for turn := 1; turn <= a.MaxTurns; turn++ {
			turnErr := a.Tracer.AgentTurn(ctx, turn, func(ctx context.Context) error {
				messages := buildMessages(convo)
				res, err := a.Tracer.LLMCall(ctx, "ollama.chat", a.Model, messages, cacheMeta, func(ctx context.Context) (domain.LLMResult, error) {
					return a.LLM.Chat(ctx, messages, tools)
				})
				if err != nil {
					return err
				}
				total = total.Add(res.Usage)
				cacheMeta["total_prompt_tokens"] = total.PromptTokens
				cacheMeta["last_cache_read_tokens"] = res.Usage.CachedPromptTokens

				asst := domain.ChatMessage{Role: "assistant", Content: res.Message.Content, ToolCalls: res.Message.ToolCalls}
				convo = append(convo, asst)

				if len(res.Message.ToolCalls) == 0 {
					final = res.Message.Content
					return nil
				}

				for _, tc := range res.Message.ToolCalls {
					out, execErr := a.runTool(ctx, tc, opts)
					if execErr != nil {
						out = fmt.Sprintf(`{"error":%q}`, execErr.Error())
					}
					convo = append(convo, domain.ChatMessage{
						Role: "tool", ToolCallID: tc.ID, Name: tc.Name, Content: out,
					})
				}
				return nil
			})
			if turnErr != nil {
				return turnErr
			}
			if final != "" {
				return nil
			}
		}
		final = "Agent stopped: max turns reached."
		return nil
	})
	if err != nil {
		return "", err
	}
	slog.Info("agent.done",
		"session_id", opts.SessionID,
		"prompt_tokens", total.PromptTokens,
		"completion_tokens", total.CompletionTokens,
	)
	return final, nil
}

func (a *Agent) runTool(ctx context.Context, tc domain.ToolCall, opts RunOpts) (string, error) {
	if tc.Name == "Task" {
		if opts.Depth >= 1 {
			return "", fmt.Errorf("nested Task not allowed")
		}
		prompt, _ := tc.Arguments["prompt"].(string)
		maxTurns := a.MaxTurns / 2
		if v, ok := tc.Arguments["max_turns"].(float64); ok && int(v) > 0 {
			maxTurns = int(v)
		}
		sub := *a
		sub.MaxTurns = maxTurns
		sub.Warm = false
		subOpts := opts
		subOpts.Depth = opts.Depth + 1
		subOpts.WorkflowName = "nl-lf-subagent"
		answer, err := sub.Run(ctx, prompt, subOpts)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"answer": answer})
		return string(b), nil
	}
	return a.Tracer.ToolCall(ctx, tc.Name, tc.Arguments, func(ctx context.Context) (string, error) {
		return a.Tools.Dispatch(ctx, tc.Name, tc.Arguments)
	})
}

func buildMessages(convo []domain.ChatMessage) []domain.ChatMessage {
	out := []domain.ChatMessage{{Role: "system", Content: systemPrompt}}
	out = append(out, convo...)
	return out
}
