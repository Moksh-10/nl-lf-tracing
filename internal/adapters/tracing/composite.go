package tracing

import (
	"context"

	"github.com/Moksh-10/nl-lf-tracing/internal/domain"
	"github.com/Moksh-10/nl-lf-tracing/internal/ports"
)

type Composite struct {
	items []ports.Tracer
}

func NewComposite(items ...ports.Tracer) *Composite {
	return &Composite{items: items}
}

func (c *Composite) Workflow(ctx context.Context, name string, meta ports.WorkflowMeta, input any, fn func(context.Context) error) error {
	return c.nest(ctx, 0, func(t ports.Tracer, next func(context.Context) error) error {
		return t.Workflow(ctx, name, meta, input, next)
	}, fn)
}

func (c *Composite) AgentTurn(ctx context.Context, turn int, fn func(context.Context) error) error {
	return c.nest(ctx, 0, func(t ports.Tracer, next func(context.Context) error) error {
		return t.AgentTurn(ctx, turn, next)
	}, fn)
}

func (c *Composite) LLMCall(
	ctx context.Context,
	name, model string,
	messages []domain.ChatMessage,
	meta map[string]any,
	fn func(context.Context) (domain.LLMResult, error),
) (domain.LLMResult, error) {
	return c.nestResult(ctx, 0, func(t ports.Tracer, next func(context.Context) (domain.LLMResult, error)) (domain.LLMResult, error) {
		return t.LLMCall(ctx, name, model, messages, meta, next)
	}, fn)
}

func (c *Composite) ToolCall(ctx context.Context, toolName string, args map[string]any, fn func(context.Context) (string, error)) (string, error) {
	return c.nestString(ctx, 0, func(t ports.Tracer, next func(context.Context) (string, error)) (string, error) {
		return t.ToolCall(ctx, toolName, args, next)
	}, fn)
}

func (c *Composite) Flush(ctx context.Context) error {
	for _, t := range c.items {
		_ = t.Flush(ctx)
	}
	return nil
}

func (c *Composite) Shutdown(ctx context.Context) error {
	for _, t := range c.items {
		_ = t.Shutdown(ctx)
	}
	return nil
}

func (c *Composite) nest(
	ctx context.Context,
	idx int,
	wrap func(ports.Tracer, func(context.Context) error) error,
	fn func(context.Context) error,
) error {
	if len(c.items) == 0 {
		return fn(ctx)
	}
	if idx >= len(c.items)-1 {
		return wrap(c.items[idx], fn)
	}
	return wrap(c.items[idx], func(ctx context.Context) error {
		return c.nest(ctx, idx+1, wrap, fn)
	})
}

func (c *Composite) nestResult(
	ctx context.Context,
	idx int,
	wrap func(ports.Tracer, func(context.Context) (domain.LLMResult, error)) (domain.LLMResult, error),
	fn func(context.Context) (domain.LLMResult, error),
) (domain.LLMResult, error) {
	if len(c.items) == 0 {
		return fn(ctx)
	}
	if idx >= len(c.items)-1 {
		return wrap(c.items[idx], fn)
	}
	return wrap(c.items[idx], func(ctx context.Context) (domain.LLMResult, error) {
		return c.nestResult(ctx, idx+1, wrap, fn)
	})
}

func (c *Composite) nestString(
	ctx context.Context,
	idx int,
	wrap func(ports.Tracer, func(context.Context) (string, error)) (string, error),
	fn func(context.Context) (string, error),
) (string, error) {
	if len(c.items) == 0 {
		return fn(ctx)
	}
	if idx >= len(c.items)-1 {
		return wrap(c.items[idx], fn)
	}
	return wrap(c.items[idx], func(ctx context.Context) (string, error) {
		return c.nestString(ctx, idx+1, wrap, fn)
	})
}

type Noop struct{}

func (Noop) Workflow(ctx context.Context, _ string, _ ports.WorkflowMeta, _ any, fn func(context.Context) error) error {
	return fn(ctx)
}
func (Noop) AgentTurn(ctx context.Context, _ int, fn func(context.Context) error) error { return fn(ctx) }
func (Noop) LLMCall(ctx context.Context, _, _ string, _ []domain.ChatMessage, _ map[string]any, fn func(context.Context) (domain.LLMResult, error)) (domain.LLMResult, error) {
	return fn(ctx)
}
func (Noop) ToolCall(ctx context.Context, _ string, _ map[string]any, fn func(context.Context) (string, error)) (string, error) {
	return fn(ctx)
}
func (Noop) Flush(context.Context) error    { return nil }
func (Noop) Shutdown(context.Context) error { return nil }
