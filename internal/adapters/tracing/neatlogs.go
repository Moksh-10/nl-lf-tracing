package tracing

import (
	"context"
	"encoding/json"
	"fmt"

	nl "github.com/neatlogs/neatlogs-go"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Moksh-10/nl-lf-tracing/internal/domain"
	"github.com/Moksh-10/nl-lf-tracing/internal/ports"
)

type Neatlogs struct {
	shutdown func(context.Context) error
}

func InitNeatlogs(ctx context.Context, apiKey, endpoint, workflowName string) (*Neatlogs, error) {
	cfg := nl.Config{
		APIKey:       apiKey,
		WorkflowName: workflowName,
	}
	if endpoint != "" {
		cfg.Endpoint = endpoint
	}
	shutdown, err := nl.Init(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Neatlogs{shutdown: shutdown}, nil
}

func (n *Neatlogs) Workflow(ctx context.Context, name string, meta ports.WorkflowMeta, input any, fn func(context.Context) error) error {
	ctx = nl.Identify(ctx, nl.IdentifyOptions{
		SessionID:          meta.SessionID,
		EndUserID:          meta.UserID,
		SessionFeatureName: "nl-lf-agent",
		SessionEntryPoint:  "cli",
	})
	ctx, root, end := nl.Trace(ctx, name)
	defer end()
	_ = nl.SetTraceInput(root, input)
	err := fn(ctx)
	out := map[string]any{"status": "ok"}
	if err != nil {
		out = map[string]any{"error": err.Error()}
	}
	_ = nl.SetTraceOutput(root, out)
	return err
}

func (n *Neatlogs) AgentTurn(ctx context.Context, turn int, fn func(context.Context) error) error {
	ctx, _, end := nl.StartSpan(ctx, fmt.Sprintf("agent.turn.%d", turn), "agent",
		attribute.Int("turn", turn),
	)
	defer end()
	return fn(ctx)
}

func (n *Neatlogs) LLMCall(
	ctx context.Context,
	name, model string,
	messages []domain.ChatMessage,
	meta map[string]any,
	fn func(context.Context) (domain.LLMResult, error),
) (domain.LLMResult, error) {
	msgs := make([]nl.LLMMessage, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, nl.LLMMessage{Role: m.Role, Content: m.Content})
	}
	ctx, llm := nl.StartLLMSpan(ctx, nl.LLMCallOptions{
		Name:     name,
		Provider: "ollama",
		Model:    model,
		Messages: msgs,
	})
	defer llm.End()
	res, err := fn(ctx)
	if err != nil {
		llm.SetError(err)
		return res, err
	}
	llm.SetOutputMessage("assistant", res.Message.Content)
	llm.SetUsage(res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens)
	llm.SetFinishReason("stop")
	_ = meta
	return res, nil
}

func (n *Neatlogs) ToolCall(ctx context.Context, toolName string, args map[string]any, fn func(context.Context) (string, error)) (string, error) {
	in, _ := json.Marshal(args)
	ctx, span, end := nl.StartSpan(ctx, "tool."+toolName, "tool",
		attribute.String("neatlogs.tool.name", toolName),
		attribute.String("neatlogs.input.value", string(in)),
	)
	defer end()
	out, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		errOut, _ := json.Marshal(map[string]any{"error": err.Error()})
		span.SetAttributes(attribute.String("neatlogs.output.value", string(errOut)))
		return out, err
	}
	span.SetAttributes(attribute.String("neatlogs.output.value", truncate(out, 12000)))
	return out, nil
}

func (n *Neatlogs) Flush(ctx context.Context) error {
	nl.Flush(ctx)
	return nil
}

func (n *Neatlogs) Shutdown(ctx context.Context) error {
	if n.shutdown != nil {
		return n.shutdown(ctx)
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
