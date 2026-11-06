package bootstrap

import (
	"context"
	"log/slog"

	"github.com/Moksh-10/nl-lf-tracing/internal/adapters/llm"
	"github.com/Moksh-10/nl-lf-tracing/internal/adapters/tools"
	"github.com/Moksh-10/nl-lf-tracing/internal/adapters/tracing"
	"github.com/Moksh-10/nl-lf-tracing/internal/application"
	"github.com/Moksh-10/nl-lf-tracing/internal/config"
	"github.com/Moksh-10/nl-lf-tracing/internal/ports"
)

type App struct {
	Agent  *application.Agent
	Tracer ports.Tracer
}

func New(ctx context.Context, cfg config.Settings) (*App, error) {
	var tracers []ports.Tracer

	if cfg.NeatlogsEnabled() {
		nl, err := tracing.InitNeatlogs(ctx, cfg.NeatlogsAPIKey, cfg.NeatlogsEndpoint, "local-ollama-agent")
		if err != nil {
			return nil, err
		}
		tracers = append(tracers, nl)
		slog.Info("tracing.neatlogs.enabled", "endpoint", cfg.NeatlogsEndpoint)
	}

	if cfg.LangfuseEnabled() {
		lf, err := tracing.InitLangfuse(ctx, cfg.LangfusePublicKey, cfg.LangfuseSecretKey, cfg.LangfuseBaseURL, cfg.TracingEnv, "nl-lf-tracing@0.1.0")
		if err != nil {
			return nil, err
		}
		tracers = append(tracers, lf)
		slog.Info("tracing.langfuse.enabled", "base_url", cfg.LangfuseBaseURL)
	}

	var tracer ports.Tracer = tracing.Noop{}
	switch len(tracers) {
	case 0:
		slog.Warn("tracing.disabled")
	case 1:
		tracer = tracers[0]
	default:
		tracer = tracing.NewComposite(tracers...)
	}

	ollama := llm.NewOllama(cfg.OllamaBaseURL, cfg.OllamaModel, cfg.OllamaKeepAlive)
	toolReg := &tools.Registry{Root: cfg.Workspace, Tavily: cfg.TavilyAPIKey}

	agent := &application.Agent{
		LLM:      ollama,
		Tracer:   tracer,
		Tools:    toolReg,
		Model:    cfg.OllamaModel,
		MaxTurns: cfg.MaxTurns,
		Warm:     true,
	}
	return &App{Agent: agent, Tracer: tracer}, nil
}
