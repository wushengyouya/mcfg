package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mcfg/internal/model"
)

var pages = []string{"Overview", "Models", "MCP Servers", "Sync Preview", "Backups"}

const (
	pageOverview = iota
	pageModels
	pageMCPServers
	pageSyncPreview
	pageBackups
)

const (
	confirmSync        = "sync"
	confirmRestore     = "restore"
	confirmDeleteModel = "delete_model"
	confirmDeleteMCP   = "delete_mcp"
)

// BackupItem 表示 TUI 中展示的一条备份摘要。
type BackupItem struct {
	ID        string
	CreatedAt string
	Reason    string
	Corrupted bool
}

// ModelFormInput 表示 TUI 中模型表单的输入值。
type ModelFormInput struct {
	Name        string
	BaseURL     string
	Model       string
	AuthToken   string
	Description string
}

// MCPFormInput 表示 TUI 中 MCP 表单的输入值。
type MCPFormInput struct {
	Name        string
	Command     string
	Args        []string
	Env         map[string]string
	Description string
}

// Snapshot 表示 TUI 当前展示所需的只读视图数据。
type Snapshot struct {
	CurrentModelID   string
	CurrentModelName string
	EnabledMCPIDs    []string
	EnabledMCPCount  int
	LastSyncResult   string
	LockStatus       string
	TargetStatus     string
	Models           []model.ModelProfile
	MCPServers       []model.MCPServer
}

// Controller 定义 TUI 与命令服务层之间的交互接口。
type Controller interface {
	Refresh() (Snapshot, error)
	UseModel(id string) (Snapshot, error)
	ToggleMCP(id string) (Snapshot, error)
	SyncPreview() ([]string, error)
	ListBackups() ([]BackupItem, error)
	SyncApply() (Snapshot, []string, error)
	RestoreBackup(id string) (Snapshot, []BackupItem, error)
	AddModel(input ModelFormInput) (Snapshot, error)
	EditModel(id string, input ModelFormInput) (Snapshot, error)
	RemoveModel(id string) (Snapshot, error)
	AddMCP(input MCPFormInput) (Snapshot, error)
	EditMCP(id string, input MCPFormInput) (Snapshot, error)
	RemoveMCP(id string) (Snapshot, error)
}

type formField struct {
	label  string
	value  string
	secret bool
}

type formState struct {
	mode   string
	title  string
	target string
	fields []formField
	index  int
}

// App 表示交互式终端界面的状态机。
type App struct {
	index         int
	modelCursor   int
	mcpCursor     int
	backupCursor  int
	snapshot      Snapshot
	controller    Controller
	driftPaths    []string
	backups       []BackupItem
	form          *formState
	confirmAction string
	confirmTarget string
	statusMessage string
	errorMessage  string
	quitting      bool
}

// New 根据初始快照和控制器创建 TUI 应用实例。
func New(snapshot Snapshot, controller Controller) App {
	snapshot.LockStatus = fallback(snapshot.LockStatus, "exclusive")
	snapshot.TargetStatus = fallback(snapshot.TargetStatus, "unknown")
	if snapshot.EnabledMCPIDs == nil {
		snapshot.EnabledMCPIDs = []string{}
	}
	return App{
		index:      0,
		snapshot:   snapshot,
		controller: controller,
		driftPaths: []string{},
		backups:    []BackupItem{},
	}
}

// Init 实现 Bubble Tea 模型初始化接口。
func (a App) Init() tea.Cmd {
	return nil
}

// Update 实现 Bubble Tea 模型更新接口。
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if a.form != nil {
			return a.handleForm(msg)
		}
		if a.confirmAction != "" {
			return a.handleConfirm(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			a.quitting = true
			return a, tea.Quit
		case "l", "right":
			a.switchPage(a.index + 1)
		case "h", "left":
			a.switchPage(a.index - 1)
		case "j", "down":
			if !a.moveCursor(1) {
				a.switchPage(a.index + 1)
			}
		case "k", "up":
			if !a.moveCursor(-1) {
				a.switchPage(a.index - 1)
			}
		case "u":
			a.useSelectedModel()
		case " ":
			a.toggleSelectedMCP()
		case "s":
			a.switchPage(pageSyncPreview)
			a.loadSyncPreview()
		case "r":
			a.refreshCurrentPage()
		case "enter":
			a.startConfirm()
		case "a":
			a.startAddForm()
		case "e":
			a.startEditForm()
		case "d":
			a.startDeleteConfirm()
		}
	}
	return a, nil
}

// View 实现 Bubble Tea 模型渲染接口。
func (a App) View() string {
	if a.quitting {
		return "Bye."
	}

	navItems := make([]string, 0, len(pages))
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	for i, page := range pages {
		if i == a.index {
			navItems = append(navItems, activeStyle.Render("> "+page))
			continue
		}
		navItems = append(navItems, inactiveStyle.Render("  "+page))
	}

	left := lipgloss.NewStyle().
		Width(18).
		Padding(1, 1).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		Render(strings.Join(navItems, "\n"))

	rightContent := a.pageView()
	if a.form != nil {
		rightContent = a.formView()
	}
	right := lipgloss.NewStyle().
		Padding(1, 2).
		Width(78).
		Render(rightContent)

	statusParts := []string{
		fmt.Sprintf("Lock: %s", a.snapshot.LockStatus),
		fmt.Sprintf("Target: %s", a.snapshot.TargetStatus),
		fmt.Sprintf("Last Sync: %s", fallback(a.snapshot.LastSyncResult, "never")),
		"Keys: h/l j/k a e d u space s r q",
	}
	if a.statusMessage != "" {
		statusParts = append(statusParts, "Info: "+a.statusMessage)
	}
	if a.errorMessage != "" {
		statusParts = append(statusParts, "Error: "+a.errorMessage)
	}
	if a.confirmAction != "" {
		statusParts = append(statusParts, "Confirm: "+a.confirmAction+" (enter/y confirm, esc/n cancel)")
	}
	if a.form != nil {
		statusParts = append(statusParts, "Form: enter/tab next, backspace delete, esc cancel")
	}
	status := lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("237")).
		Render(strings.Join(statusParts, " | "))

	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Top, left, right), status)
}

func (a App) pageView() string {
	switch a.index {
	case pageOverview:
		modelName := fallback(a.snapshot.CurrentModelName, "(none)")
		return fmt.Sprintf("Overview\n\nCurrent model: %s\nEnabled MCPs: %d\nLast sync: %s", modelName, a.snapshot.EnabledMCPCount, fallback(a.snapshot.LastSyncResult, "never"))
	case pageModels:
		if len(a.snapshot.Models) == 0 {
			return "Models\n\nNo models configured.\n\nPress `a` to add a model."
		}
		lines := []string{"Models", ""}
		for i, item := range a.snapshot.Models {
			prefix := "  "
			if i == a.modelCursor {
				prefix = "> "
			}
			label := prefix + item.Name
			if item.ID == a.snapshot.CurrentModelID {
				label += " [active]"
			}
			lines = append(lines, label)
		}
		lines = append(lines, "", "Press `a` add, `e` edit, `d` delete, `u` use.")
		return strings.Join(lines, "\n")
	case pageMCPServers:
		if len(a.snapshot.MCPServers) == 0 {
			return "MCP Servers\n\nNo MCP servers configured.\n\nPress `a` to add an MCP."
		}
		lines := []string{"MCP Servers", ""}
		for i, item := range a.snapshot.MCPServers {
			prefix := "  "
			if i == a.mcpCursor {
				prefix = "> "
			}
			state := "disabled"
			if contains(a.snapshot.EnabledMCPIDs, item.ID) {
				state = "enabled"
			}
			lines = append(lines, fmt.Sprintf("%s%s (%s) [%s]", prefix, item.Name, item.Command, state))
		}
		lines = append(lines, "", "Press `a` add, `e` edit, `d` delete, `space` toggle.")
		return strings.Join(lines, "\n")
	case pageSyncPreview:
		lines := []string{"Sync Preview", ""}
		if len(a.driftPaths) == 0 {
			lines = append(lines, "No drift loaded. Press `s` or `r` to refresh.")
		} else {
			lines = append(lines, "Managed paths changed:")
			for _, path := range a.driftPaths {
				lines = append(lines, "- "+path)
			}
			lines = append(lines, "", "Press `enter` to confirm sync.")
		}
		return strings.Join(lines, "\n")
	case pageBackups:
		lines := []string{"Backups", ""}
		if len(a.backups) == 0 {
			lines = append(lines, "No backups loaded. Press `r` to refresh.")
			return strings.Join(lines, "\n")
		}
		for i, item := range a.backups {
			prefix := "  "
			if i == a.backupCursor {
				prefix = "> "
			}
			label := fmt.Sprintf("%s%s %s %s", prefix, item.ID, item.CreatedAt, item.Reason)
			if item.Corrupted {
				label += " [corrupted]"
			}
			lines = append(lines, label)
		}
		lines = append(lines, "", "Press `enter` to confirm restore. Press `r` to refresh backups.")
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func (a App) formView() string {
	lines := []string{a.form.title, ""}
	for i, field := range a.form.fields {
		prefix := "  "
		if i == a.form.index {
			prefix = "> "
		}
		value := field.value
		if field.secret {
			value = strings.Repeat("*", len(value))
		}
		lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, field.label, value))
	}
	lines = append(lines, "", "Press `enter` on the last field to submit.")
	return strings.Join(lines, "\n")
}

func (a *App) switchPage(index int) {
	a.index = (index + len(pages)) % len(pages)
	a.clearTransientState()
	switch a.index {
	case pageSyncPreview:
		a.loadSyncPreview()
	case pageBackups:
		a.loadBackups()
	}
}

func (a *App) moveCursor(delta int) bool {
	switch a.index {
	case pageModels:
		if len(a.snapshot.Models) == 0 {
			return false
		}
		a.modelCursor = wrapIndex(a.modelCursor+delta, len(a.snapshot.Models))
		return true
	case pageMCPServers:
		if len(a.snapshot.MCPServers) == 0 {
			return false
		}
		a.mcpCursor = wrapIndex(a.mcpCursor+delta, len(a.snapshot.MCPServers))
		return true
	case pageBackups:
		if len(a.backups) == 0 {
			return false
		}
		a.backupCursor = wrapIndex(a.backupCursor+delta, len(a.backups))
		return true
	default:
		return false
	}
}

func (a *App) refreshCurrentPage() {
	a.clearTransientState()
	if a.controller != nil {
		snapshot, err := a.controller.Refresh()
		if err != nil {
			a.errorMessage = err.Error()
			return
		}
		a.snapshot = snapshot
	}
	switch a.index {
	case pageSyncPreview:
		a.loadSyncPreview()
	case pageBackups:
		a.loadBackups()
	default:
		a.statusMessage = "refreshed"
	}
}

func (a *App) useSelectedModel() {
	if a.index != pageModels || len(a.snapshot.Models) == 0 || a.controller == nil {
		return
	}
	snapshot, err := a.controller.UseModel(a.snapshot.Models[a.modelCursor].ID)
	if err != nil {
		a.errorMessage = err.Error()
		return
	}
	a.snapshot = snapshot
	a.statusMessage = "model switched"
	a.errorMessage = ""
}

func (a *App) toggleSelectedMCP() {
	if a.index != pageMCPServers || len(a.snapshot.MCPServers) == 0 || a.controller == nil {
		return
	}
	snapshot, err := a.controller.ToggleMCP(a.snapshot.MCPServers[a.mcpCursor].ID)
	if err != nil {
		a.errorMessage = err.Error()
		return
	}
	a.snapshot = snapshot
	a.statusMessage = "mcp toggled"
	a.errorMessage = ""
}

func (a *App) loadSyncPreview() {
	if a.controller == nil {
		return
	}
	drift, err := a.controller.SyncPreview()
	if err != nil {
		a.errorMessage = err.Error()
		return
	}
	a.driftPaths = drift
	a.statusMessage = "sync preview refreshed"
	a.errorMessage = ""
}

func (a *App) loadBackups() {
	if a.controller == nil {
		return
	}
	backups, err := a.controller.ListBackups()
	if err != nil {
		a.errorMessage = err.Error()
		return
	}
	a.backups = backups
	if len(a.backups) == 0 {
		a.backupCursor = 0
	} else {
		a.backupCursor = wrapIndex(a.backupCursor, len(a.backups))
	}
	a.statusMessage = "backups refreshed"
	a.errorMessage = ""
}

func (a *App) startConfirm() {
	switch a.index {
	case pageSyncPreview:
		if len(a.driftPaths) > 0 {
			a.confirmAction = confirmSync
		}
	case pageBackups:
		if len(a.backups) > 0 {
			a.confirmAction = confirmRestore
			a.confirmTarget = a.backups[a.backupCursor].ID
		}
	}
}

func (a *App) startDeleteConfirm() {
	switch a.index {
	case pageModels:
		if len(a.snapshot.Models) > 0 {
			a.confirmAction = confirmDeleteModel
			a.confirmTarget = a.snapshot.Models[a.modelCursor].ID
		}
	case pageMCPServers:
		if len(a.snapshot.MCPServers) > 0 {
			a.confirmAction = confirmDeleteMCP
			a.confirmTarget = a.snapshot.MCPServers[a.mcpCursor].ID
		}
	}
}

func (a App) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		a.confirmAction = ""
		a.confirmTarget = ""
		a.statusMessage = "cancelled"
		a.errorMessage = ""
	case "enter", "y":
		switch a.confirmAction {
		case confirmSync:
			a.applySync()
		case confirmRestore:
			a.restoreSelectedBackup()
		case confirmDeleteModel:
			a.deleteSelectedModel()
		case confirmDeleteMCP:
			a.deleteSelectedMCP()
		}
	}
	return a, nil
}

func (a *App) applySync() {
	if a.controller == nil {
		a.clearTransientState()
		return
	}
	snapshot, drift, err := a.controller.SyncApply()
	if err != nil {
		a.errorMessage = err.Error()
		a.confirmAction = ""
		a.confirmTarget = ""
		return
	}
	a.snapshot = snapshot
	a.driftPaths = drift
	a.statusMessage = "sync completed"
	a.errorMessage = ""
	a.confirmAction = ""
	a.confirmTarget = ""
}

func (a *App) restoreSelectedBackup() {
	if a.controller == nil || a.confirmTarget == "" {
		a.clearTransientState()
		return
	}
	snapshot, backups, err := a.controller.RestoreBackup(a.confirmTarget)
	if err != nil {
		a.errorMessage = err.Error()
		a.confirmAction = ""
		a.confirmTarget = ""
		return
	}
	a.snapshot = snapshot
	a.backups = backups
	if len(a.backups) == 0 {
		a.backupCursor = 0
	} else {
		a.backupCursor = wrapIndex(a.backupCursor, len(a.backups))
	}
	a.statusMessage = "backup restored"
	a.errorMessage = ""
	a.confirmAction = ""
	a.confirmTarget = ""
}

func (a *App) deleteSelectedModel() {
	if a.controller == nil || a.confirmTarget == "" {
		a.clearTransientState()
		return
	}
	snapshot, err := a.controller.RemoveModel(a.confirmTarget)
	if err != nil {
		a.errorMessage = err.Error()
		a.confirmAction = ""
		a.confirmTarget = ""
		return
	}
	a.snapshot = snapshot
	if len(a.snapshot.Models) == 0 {
		a.modelCursor = 0
	} else {
		a.modelCursor = wrapIndex(a.modelCursor, len(a.snapshot.Models))
	}
	a.statusMessage = "model removed"
	a.errorMessage = ""
	a.confirmAction = ""
	a.confirmTarget = ""
}

func (a *App) deleteSelectedMCP() {
	if a.controller == nil || a.confirmTarget == "" {
		a.clearTransientState()
		return
	}
	snapshot, err := a.controller.RemoveMCP(a.confirmTarget)
	if err != nil {
		a.errorMessage = err.Error()
		a.confirmAction = ""
		a.confirmTarget = ""
		return
	}
	a.snapshot = snapshot
	if len(a.snapshot.MCPServers) == 0 {
		a.mcpCursor = 0
	} else {
		a.mcpCursor = wrapIndex(a.mcpCursor, len(a.snapshot.MCPServers))
	}
	a.statusMessage = "mcp removed"
	a.errorMessage = ""
	a.confirmAction = ""
	a.confirmTarget = ""
}

func (a *App) startAddForm() {
	switch a.index {
	case pageModels:
		a.form = &formState{
			mode:  "model_add",
			title: "Add Model",
			fields: []formField{
				{label: "Name"},
				{label: "Base URL"},
				{label: "Model"},
				{label: "Auth Token", secret: true},
				{label: "Description"},
			},
		}
	case pageMCPServers:
		a.form = &formState{
			mode:  "mcp_add",
			title: "Add MCP",
			fields: []formField{
				{label: "Name"},
				{label: "Command"},
				{label: "Args (comma separated)"},
				{label: "Env (KEY=VALUE,comma separated)"},
				{label: "Description"},
			},
		}
	}
	a.statusMessage = ""
	a.errorMessage = ""
	a.confirmAction = ""
	a.confirmTarget = ""
}

func (a *App) startEditForm() {
	switch a.index {
	case pageModels:
		if len(a.snapshot.Models) == 0 {
			return
		}
		item := a.snapshot.Models[a.modelCursor]
		a.form = &formState{
			mode:   "model_edit",
			title:  "Edit Model",
			target: item.ID,
			fields: []formField{
				{label: "Name", value: item.Name},
				{label: "Base URL", value: item.Env["ANTHROPIC_BASE_URL"]},
				{label: "Model", value: item.Env["ANTHROPIC_MODEL"]},
				{label: "Auth Token", value: item.Env["ANTHROPIC_AUTH_TOKEN"], secret: true},
				{label: "Description", value: item.Description},
			},
		}
	case pageMCPServers:
		if len(a.snapshot.MCPServers) == 0 {
			return
		}
		item := a.snapshot.MCPServers[a.mcpCursor]
		a.form = &formState{
			mode:   "mcp_edit",
			title:  "Edit MCP",
			target: item.ID,
			fields: []formField{
				{label: "Name", value: item.Name},
				{label: "Command", value: item.Command},
				{label: "Args (comma separated)", value: strings.Join(item.Args, ",")},
				{label: "Env (KEY=VALUE,comma separated)", value: formatEnv(item.Env)},
				{label: "Description", value: item.Description},
			},
		}
	}
	a.statusMessage = ""
	a.errorMessage = ""
	a.confirmAction = ""
	a.confirmTarget = ""
}

func (a App) handleForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.form = nil
		a.statusMessage = "cancelled"
		a.errorMessage = ""
		return a, nil
	case "tab", "enter":
		if a.form.index == len(a.form.fields)-1 {
			a.submitForm()
			return a, nil
		}
		a.form.index++
		return a, nil
	case "backspace":
		field := &a.form.fields[a.form.index]
		if len(field.value) > 0 {
			field.value = field.value[:len(field.value)-1]
		}
		return a, nil
	case " ":
		a.form.fields[a.form.index].value += " "
		return a, nil
	}
	if len(msg.Runes) > 0 {
		a.form.fields[a.form.index].value += string(msg.Runes)
	}
	return a, nil
}

func (a *App) submitForm() {
	if a.controller == nil || a.form == nil {
		return
	}
	var (
		snapshot Snapshot
		err      error
	)
	switch a.form.mode {
	case "model_add":
		snapshot, err = a.controller.AddModel(ModelFormInput{
			Name:        a.form.fields[0].value,
			BaseURL:     a.form.fields[1].value,
			Model:       a.form.fields[2].value,
			AuthToken:   a.form.fields[3].value,
			Description: a.form.fields[4].value,
		})
		if err == nil {
			a.statusMessage = "model added"
		}
	case "model_edit":
		snapshot, err = a.controller.EditModel(a.form.target, ModelFormInput{
			Name:        a.form.fields[0].value,
			BaseURL:     a.form.fields[1].value,
			Model:       a.form.fields[2].value,
			AuthToken:   a.form.fields[3].value,
			Description: a.form.fields[4].value,
		})
		if err == nil {
			a.statusMessage = "model updated"
		}
	case "mcp_add":
		snapshot, err = a.controller.AddMCP(MCPFormInput{
			Name:        a.form.fields[0].value,
			Command:     a.form.fields[1].value,
			Args:        parseCSV(a.form.fields[2].value),
			Env:         parseEnvCSV(a.form.fields[3].value),
			Description: a.form.fields[4].value,
		})
		if err == nil {
			a.statusMessage = "mcp added"
		}
	case "mcp_edit":
		snapshot, err = a.controller.EditMCP(a.form.target, MCPFormInput{
			Name:        a.form.fields[0].value,
			Command:     a.form.fields[1].value,
			Args:        parseCSV(a.form.fields[2].value),
			Env:         parseEnvCSV(a.form.fields[3].value),
			Description: a.form.fields[4].value,
		})
		if err == nil {
			a.statusMessage = "mcp updated"
		}
	}
	if err != nil {
		a.errorMessage = err.Error()
		return
	}
	a.snapshot = snapshot
	a.errorMessage = ""
	a.form = nil
}

func (a *App) clearTransientState() {
	a.statusMessage = ""
	a.errorMessage = ""
	a.confirmAction = ""
	a.confirmTarget = ""
	a.form = nil
}

func wrapIndex(index, length int) int {
	if length == 0 {
		return 0
	}
	return (index + length) % length
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func parseCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func parseEnvCSV(value string) map[string]string {
	env := map[string]string{}
	for _, item := range parseCSV(value) {
		key, rawValue, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		env[strings.TrimSpace(key)] = strings.TrimSpace(rawValue)
	}
	return env
}

func formatEnv(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	parts := make([]string, 0, len(env))
	for key, value := range env {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ",")
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
