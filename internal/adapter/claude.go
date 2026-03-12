package adapter

import (
	"encoding/json"
	"maps"

	"mcfg/internal/model"
)

type Claude struct {
	HomeDir string
}

func (c Claude) RenderSettings(existing []byte, currentModel *model.ModelProfile) ([]byte, error) {
	root := map[string]any{}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, err
		}
	}
	env := getObject(root, "env")
	delete(env, "ANTHROPIC_AUTH_TOKEN")
	delete(env, "ANTHROPIC_BASE_URL")
	delete(env, "ANTHROPIC_MODEL")
	if currentModel != nil {
		env["ANTHROPIC_AUTH_TOKEN"] = currentModel.Env["ANTHROPIC_AUTH_TOKEN"]
		env["ANTHROPIC_BASE_URL"] = currentModel.Env["ANTHROPIC_BASE_URL"]
		env["ANTHROPIC_MODEL"] = currentModel.Env["ANTHROPIC_MODEL"]
	}
	root["env"] = env
	return json.MarshalIndent(root, "", "  ")
}

func (c Claude) RenderClaudeJSON(existing []byte, servers []model.MCPServer) ([]byte, error) {
	root := map[string]any{}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, err
		}
	}
	homeNode := getObject(root, c.HomeDir)
	mcpServers := map[string]any{}
	for _, server := range servers {
		entry := map[string]any{
			"type":    server.Transport,
			"command": server.Command,
		}
		if len(server.Args) > 0 {
			entry["args"] = server.Args
		}
		if len(server.Env) > 0 {
			entry["env"] = maps.Clone(server.Env)
		}
		mcpServers[server.Name] = entry
	}
	homeNode["mcpServers"] = mcpServers
	root[c.HomeDir] = homeNode
	return json.MarshalIndent(root, "", "  ")
}

func SettingsManagedValues(data []byte) (map[string]string, error) {
	var payload struct {
		Env map[string]string `json:"env"`
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
	}
	return map[string]string{
		"env.ANTHROPIC_AUTH_TOKEN": payload.Env["ANTHROPIC_AUTH_TOKEN"],
		"env.ANTHROPIC_BASE_URL":   payload.Env["ANTHROPIC_BASE_URL"],
		"env.ANTHROPIC_MODEL":      payload.Env["ANTHROPIC_MODEL"],
	}, nil
}

func ClaudeManagedValues(data []byte, homeDir string) (map[string]any, error) {
	root := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, err
		}
	}
	homeNode := getObject(root, homeDir)
	mcpServers, ok := homeNode["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = map[string]any{}
	}
	return map[string]any{"<home>.mcpServers": mcpServers}, nil
}

func getObject(root map[string]any, key string) map[string]any {
	if value, ok := root[key].(map[string]any); ok {
		return value
	}
	value := map[string]any{}
	root[key] = value
	return value
}

func DiffManagedPaths(actualSettings, desiredSettings []byte, actualClaude, desiredClaude []byte, homeDir string) ([]string, error) {
	changes := []string{}

	actualEnv, err := SettingsManagedValues(actualSettings)
	if err != nil {
		return nil, err
	}
	desiredEnv, err := SettingsManagedValues(desiredSettings)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"env.ANTHROPIC_AUTH_TOKEN", "env.ANTHROPIC_BASE_URL", "env.ANTHROPIC_MODEL"} {
		if actualEnv[key] != desiredEnv[key] {
			changes = append(changes, key)
		}
	}

	actualMCPs, err := ClaudeManagedValues(actualClaude, homeDir)
	if err != nil {
		return nil, err
	}
	desiredMCPs, err := ClaudeManagedValues(desiredClaude, homeDir)
	if err != nil {
		return nil, err
	}
	actualData, _ := json.Marshal(actualMCPs["<home>.mcpServers"])
	desiredData, _ := json.Marshal(desiredMCPs["<home>.mcpServers"])
	if string(actualData) != string(desiredData) {
		changes = append(changes, "<home>.mcpServers")
	}
	return changes, nil
}
