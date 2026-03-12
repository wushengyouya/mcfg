package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/model"
	"mcfg/internal/service"
)

func TestMCPAdd_Success(t *testing.T) {
	store := &memoryStore{cfg: model.NewConfigRoot()}
	svc := service.NewMCPService(store, fixedClock{}, stubIDGen{id: "01HQXBG84ESB7XJQ9WAAYH54AM"})

	server, err := svc.Add(context.Background(), service.MCPAddInput{
		Name:    "filesystem",
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{"ROOT": "/tmp"},
	})
	require.NoError(t, err)
	require.Equal(t, "stdio", server.Transport)
	require.Equal(t, "/tmp", server.Env["ROOT"])
}

func TestMCPRemove_Enabled_WithForce(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.MCPServers = append(cfg.MCPServers, validMCP("01HQXBG84ESB7XJQ9WAAYH54AM"))
	cfg.ClaudeBinding.EnabledMCPIDs = []string{cfg.MCPServers[0].ID}

	store := &memoryStore{cfg: cfg}
	svc := service.NewMCPService(store, fixedClock{}, stubIDGen{})

	err := svc.Remove(context.Background(), "01HQXBG8", true)
	require.NoError(t, err)
	require.Empty(t, store.cfg.MCPServers)
	require.Empty(t, store.cfg.ClaudeBinding.EnabledMCPIDs)
}

func TestMCPEnable_AlreadyEnabled(t *testing.T) {
	cfg := model.NewConfigRoot()
	cfg.MCPServers = append(cfg.MCPServers, validMCP("01HQXBG84ESB7XJQ9WAAYH54AM"))
	cfg.ClaudeBinding.EnabledMCPIDs = []string{cfg.MCPServers[0].ID}

	store := &memoryStore{cfg: cfg}
	svc := service.NewMCPService(store, fixedClock{}, stubIDGen{})

	alreadyEnabled, _, err := svc.Enable(context.Background(), "01HQXBG8")
	require.NoError(t, err)
	require.True(t, alreadyEnabled)
}

func validMCP(id string) model.MCPServer {
	return model.MCPServer{
		ID:        id,
		Name:      "filesystem",
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Source:    model.SourceManual,
		CreatedAt: "2026-03-10T10:00:00Z",
		UpdatedAt: "2026-03-10T10:00:00Z",
	}
}
