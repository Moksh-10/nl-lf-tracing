package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Settings struct {
	OllamaBaseURL   string
	OllamaModel     string
	OllamaKeepAlive string
	Workspace       string

	LangfusePublicKey string
	LangfuseSecretKey string
	LangfuseBaseURL   string

	NeatlogsAPIKey   string
	NeatlogsEndpoint string

	TavilyAPIKey string

	TracingLangfuse  bool
	TracingNeatlogs  bool
	TracingEnv       string
	MaxTurns         int
	LogLevel         string
}

func Load() (Settings, error) {
	_ = godotenv.Load()

	ws := strings.TrimSpace(os.Getenv("AGENT_WORKSPACE"))
	if ws == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Settings{}, err
		}
		ws = cwd
	}
	ws, _ = filepath.Abs(ws)

	endpoint := strings.TrimSpace(os.Getenv("NEATLOGS_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://ingest.neatlogs.com"
	}

	lfHost := strings.TrimSpace(os.Getenv("LANGFUSE_BASE_URL"))
	if lfHost == "" {
		lfHost = strings.TrimSpace(os.Getenv("LANGFUSE_HOST"))
	}
	if lfHost == "" {
		lfHost = "https://cloud.langfuse.com"
	}

	return Settings{
		OllamaBaseURL:     strings.TrimRight(env("OLLAMA_BASE_URL", "http://127.0.0.1:11434/v1"), "/"),
		OllamaModel:       env("OLLAMA_MODEL", "qwen3:1.7b"),
		OllamaKeepAlive:   env("OLLAMA_KEEP_ALIVE", "30m"),
		Workspace:         ws,
		LangfusePublicKey: strings.TrimSpace(os.Getenv("LANGFUSE_PUBLIC_KEY")),
		LangfuseSecretKey: strings.TrimSpace(os.Getenv("LANGFUSE_SECRET_KEY")),
		LangfuseBaseURL:   strings.TrimRight(lfHost, "/"),
		NeatlogsAPIKey:    strings.TrimSpace(os.Getenv("NEATLOGS_API_KEY")),
		NeatlogsEndpoint:  strings.TrimRight(endpoint, "/"),
		TavilyAPIKey:      strings.TrimSpace(os.Getenv("TAVILY_API_KEY")),
		TracingLangfuse:   envBool("TRACING_LANGFUSE", true),
		TracingNeatlogs:   envBool("TRACING_NEATLOGS", true),
		TracingEnv:        env("TRACING_ENVIRONMENT", "local"),
		MaxTurns:          envInt("AGENT_MAX_TURNS", 12),
		LogLevel:          strings.ToUpper(env("LOG_LEVEL", "INFO")),
	}, nil
}

func (s Settings) LangfuseEnabled() bool {
	return s.TracingLangfuse && s.LangfusePublicKey != "" && s.LangfuseSecretKey != ""
}

func (s Settings) NeatlogsEnabled() bool {
	return s.TracingNeatlogs && s.NeatlogsAPIKey != ""
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
