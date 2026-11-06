package domain

func ToolDefinitions() []ToolDef {
	return []ToolDef{
		{
			Name:        "Read",
			Description: "Read a file from the workspace.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":   map[string]any{"type": "string"},
					"offset": map[string]any{"type": "integer"},
					"limit":  map[string]any{"type": "integer"},
				},
				"required": []any{"path"},
			}),
		},
		{
			Name:        "Write",
			Description: "Create or overwrite a file.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":     map[string]any{"type": "string"},
					"contents": map[string]any{"type": "string"},
				},
				"required": []any{"path", "contents"},
			}),
		},
		{
			Name:        "Edit",
			Description: "Search and replace in a file.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":         map[string]any{"type": "string"},
					"old_string":   map[string]any{"type": "string"},
					"new_string":   map[string]any{"type": "string"},
					"replace_all":  map[string]any{"type": "boolean"},
				},
				"required": []any{"path", "old_string", "new_string"},
			}),
		},
		{
			Name:        "Glob",
			Description: "Find files by glob pattern.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"glob_pattern":     map[string]any{"type": "string"},
					"target_directory": map[string]any{"type": "string"},
				},
				"required": []any{"glob_pattern"},
			}),
		},
		{
			Name:        "Grep",
			Description: "Search file contents with regex.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern":    map[string]any{"type": "string"},
					"path":       map[string]any{"type": "string"},
					"glob":       map[string]any{"type": "string"},
					"head_limit": map[string]any{"type": "integer"},
				},
				"required": []any{"pattern"},
			}),
		},
		{
			Name:        "Bash",
			Description: "Run a shell command in the workspace.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command":         map[string]any{"type": "string"},
					"timeout_seconds": map[string]any{"type": "integer"},
				},
				"required": []any{"command"},
			}),
		},
		{
			Name:        "WebFetch",
			Description: "Fetch URL content via Tavily extract.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string"},
				},
				"required": []any{"url"},
			}),
		},
		{
			Name:        "WebSearch",
			Description: "Search the web via Tavily.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":       map[string]any{"type": "string"},
					"max_results": map[string]any{"type": "integer"},
				},
				"required": []any{"query"},
			}),
		},
		{
			Name:        "Task",
			Description: "Spawn a short sub-agent for a focused subtask.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt":    map[string]any{"type": "string"},
					"max_turns": map[string]any{"type": "integer"},
				},
				"required": []any{"prompt"},
			}),
		},
		{
			Name:        "TodoWrite",
			Description: "Replace the in-memory todo list.",
			Parameters: obj(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"todos": map[string]any{"type": "array"},
				},
				"required": []any{"todos"},
			}),
		},
	}
}

func obj(m map[string]any) map[string]any { return m }
