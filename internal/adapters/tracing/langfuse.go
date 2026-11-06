package tracing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Moksh-10/nl-lf-tracing/internal/domain"
	"github.com/Moksh-10/nl-lf-tracing/internal/ports"
)

type Langfuse struct {
	tp       *sdktrace.TracerProvider
	tracer   trace.Tracer
	shutdown func(context.Context) error
}

func InitLangfuse(ctx context.Context, publicKey, secretKey, baseURL, environment, release string) (*Langfuse, error) {
	host := strings.TrimRight(baseURL, "/")
	endpoint := normalizeOTLPEndpoint(host + "/api/public/otel")

	auth := base64.StdEncoding.EncodeToString([]byte(publicKey + ":" + secretKey))
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization":                "Basic " + auth,
			"x-langfuse-ingestion-version": "4",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("langfuse exporter: %w", err)
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceName("nl-lf-tracing"),
		semconv.DeploymentEnvironment(environment),
		attribute.String("langfuse.environment", environment),
	}
	if release != "" {
		attrs = append(attrs, attribute.String("langfuse.release", release))
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, attrs...))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	return &Langfuse{
		tp:       tp,
		tracer:   tp.Tracer("nl-lf/agent"),
		shutdown: tp.Shutdown,
	}, nil
}

func (l *Langfuse) Workflow(ctx context.Context, name string, meta ports.WorkflowMeta, input any, fn func(context.Context) error) error {
	ctx, span := l.tracer.Start(ctx, name,
		trace.WithAttributes(
			attribute.String("langfuse.observation.type", "agent"),
			attribute.String("langfuse.trace.name", name),
			attribute.String("langfuse.session.id", meta.SessionID),
			attribute.String("langfuse.user.id", meta.UserID),
			attribute.String("langfuse.trace.tags", "agent,ollama,nl-lf-compare"),
			attribute.String("langfuse.observation.input", truncateJSON(input)),
		),
	)
	defer span.End()
	err := fn(ctx)
	out := map[string]any{"status": "ok"}
	if err != nil {
		out = map[string]any{"error": err.Error()}
		span.RecordError(err)
	}
	span.SetAttributes(attribute.String("langfuse.observation.output", truncateJSON(out)))
	return err
}

func (l *Langfuse) AgentTurn(ctx context.Context, turn int, fn func(context.Context) error) error {
	ctx, span := l.tracer.Start(ctx, fmt.Sprintf("agent.turn.%d", turn),
		trace.WithAttributes(attribute.Int("turn", turn)),
	)
	defer span.End()
	return fn(ctx)
}

func (l *Langfuse) LLMCall(
	ctx context.Context,
	name, model string,
	messages []domain.ChatMessage,
	meta map[string]any,
	fn func(context.Context) (domain.LLMResult, error),
) (domain.LLMResult, error) {
	ctx, span := l.tracer.Start(ctx, name,
		trace.WithAttributes(
			attribute.String("langfuse.observation.type", "generation"),
			attribute.String("gen_ai.system", "ollama"),
			attribute.String("gen_ai.request.model", model),
			attribute.String("langfuse.observation.model.name", model),
			attribute.String("langfuse.observation.input", truncateJSON(messages)),
			attribute.String("langfuse.trace.metadata.cache", truncateJSON(meta)),
		),
	)
	defer span.End()
	res, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		return res, err
	}
	span.SetAttributes(
		attribute.String("langfuse.observation.output", truncateJSON(res.Message)),
		attribute.Int("gen_ai.usage.input_tokens", res.Usage.PromptTokens),
		attribute.Int("gen_ai.usage.output_tokens", res.Usage.CompletionTokens),
		attribute.String("langfuse.observation.usage_details", truncateJSON(map[string]int{
			"input": res.Usage.PromptTokens, "output": res.Usage.CompletionTokens, "total": res.Usage.TotalTokens,
			"cache_read_input_tokens": res.Usage.CachedPromptTokens,
		})),
	)
	return res, nil
}

func (l *Langfuse) ToolCall(ctx context.Context, toolName string, args map[string]any, fn func(context.Context) (string, error)) (string, error) {
	ctx, span := l.tracer.Start(ctx, "tool."+toolName,
		trace.WithAttributes(
			attribute.String("langfuse.observation.type", "tool"),
			attribute.String("langfuse.observation.metadata.tool_name", toolName),
			attribute.String("langfuse.observation.input", truncateJSON(args)),
		),
	)
	defer span.End()
	out, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
	}
	span.SetAttributes(attribute.String("langfuse.observation.output", truncateJSON(out)))
	return out, err
}

func (l *Langfuse) Flush(ctx context.Context) error {
	return l.tp.ForceFlush(ctx)
}

func (l *Langfuse) Shutdown(ctx context.Context) error {
	return l.shutdown(ctx)
}

func normalizeOTLPEndpoint(raw string) string {
	raw = strings.TrimRight(raw, "/")
	if strings.HasSuffix(raw, "/v1/traces") {
		return raw
	}
	if strings.HasSuffix(raw, "/otel") || strings.HasSuffix(raw, "/otel/v1/traces") {
		if !strings.HasSuffix(raw, "/v1/traces") {
			return strings.TrimSuffix(raw, "/") + "/v1/traces"
		}
	}
	return raw + "/v1/traces"
}

func truncateJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	const limit = 12000
	if len(b) <= limit {
		return string(b)
	}
	return string(b[:limit]) + "…"
}
