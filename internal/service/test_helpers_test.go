package service_test

func projectScopedClaudeJSON(home, mcpServersJSON string) []byte {
	return []byte(`{"projects":{"` + home + `":{"mcpServers":` + mcpServersJSON + `}}}`)
}
