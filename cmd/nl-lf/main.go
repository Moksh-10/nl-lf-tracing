package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/Moksh-10/nl-lf-tracing/internal/application"
	"github.com/Moksh-10/nl-lf-tracing/internal/bootstrap"
	"github.com/Moksh-10/nl-lf-tracing/internal/config"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	setupLogging(cfg.LogLevel)

	app, err := bootstrap.New(ctx, cfg)
	if err != nil {
		slog.Error("bootstrap", "err", err)
		os.Exit(1)
	}
	defer func() {
		_ = app.Tracer.Flush(ctx)
		_ = app.Tracer.Shutdown(ctx)
	}()

	session := envOr("NL_LF_SESSION", uuid.NewString())
	user := envOr("NL_LF_USER", "local-dev")

	if len(os.Args) > 1 {
		msg := strings.Join(os.Args[1:], " ")
		answer, err := app.Agent.Run(ctx, msg, application.RunOpts{
			SessionID: session, UserID: user, WorkflowName: "nl-lf-agent-oneshot",
		})
		if err != nil {
			slog.Error("agent", "err", err)
			os.Exit(1)
		}
		println(answer)
		return
	}

	slog.Info("repl.start", "model", cfg.OllamaModel, "workspace", cfg.Workspace, "session", session)
	println("nl-lf Go agent — traces → Langfuse Cloud + Neatlogs (when keys set)")
	println("session:", session, "| /bye to exit")
	sc := bufio.NewScanner(os.Stdin)
	for {
		print(">>> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if line == "/bye" || line == "exit" {
			break
		}
		answer, err := app.Agent.Run(ctx, line, application.RunOpts{
			SessionID: session, UserID: user, WorkflowName: "nl-lf-agent-repl",
		})
		if err != nil {
			slog.Error("agent", "err", err)
			continue
		}
		println(answer)
		println()
	}
}

func setupLogging(level string) {
	var lvl slog.Level
	switch level {
	case "DEBUG":
		lvl = slog.LevelDebug
	case "WARN":
		lvl = slog.LevelWarn
	case "ERROR":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
