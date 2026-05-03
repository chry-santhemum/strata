package strata

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var ErrPickerCancelled = errors.New("selection cancelled")

type pickerPurpose int

const (
	pickerPurposeProject pickerPurpose = iota
	pickerPurposeStart
)

type pickerMode int

const (
	pickerModeNavigate pickerMode = iota
	pickerModeInput
)

type pickerItemKind int

const (
	pickerItemStart pickerItemKind = iota
	pickerItemCreate
	pickerItemProject
)

type pickerItem struct {
	kind pickerItemKind
	path string
	text string
}

type pickerModel struct {
	store    *Store
	purpose  pickerPurpose
	mode     pickerMode
	current  string
	children []Project
	cursor   int
	input    string
	message  string
	selected string
	cancel   bool
}

func RunProjectManager(store *Store) error {
	model := newPickerModel(store, pickerPurposeProject)
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return err
	}
	if model, ok := finalModel.(pickerModel); ok && model.cancel {
		return nil
	}
	return nil
}

func RunStartPicker(store *Store) (string, error) {
	model := newPickerModel(store, pickerPurposeStart)
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", err
	}
	model, ok := finalModel.(pickerModel)
	if !ok || model.cancel || model.selected == "" {
		return "", ErrPickerCancelled
	}
	return model.selected, nil
}

func newPickerModel(store *Store, purpose pickerPurpose) pickerModel {
	model := pickerModel{
		store:   store,
		purpose: purpose,
	}
	model.reload()
	return model
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.mode == pickerModeInput {
		return m.updateInput(key)
	}
	return m.updateNavigate(key)
}

func (m pickerModel) View() string {
	var builder strings.Builder
	builder.WriteString(m.title())
	builder.WriteString("\n\n")

	if m.mode == pickerModeInput {
		fmt.Fprintf(&builder, "new project name\n> %s\n", m.input)
		if m.message != "" {
			fmt.Fprintf(&builder, "\n%s\n", m.message)
		}
		builder.WriteString("\nenter create | esc cancel | ctrl+c quit\n")
		return builder.String()
	}

	items := m.items()
	for index, item := range items {
		cursor := " "
		if index == m.cursor {
			cursor = ">"
		}
		fmt.Fprintf(&builder, "%s %s\n", cursor, item.text)
	}
	if len(items) == 0 {
		builder.WriteString("  no projects yet\n")
	}
	if m.message != "" {
		fmt.Fprintf(&builder, "\n%s\n", m.message)
	}
	builder.WriteString("\nup/down move | enter select | left back | q quit\n")
	return builder.String()
}

func (m pickerModel) updateNavigate(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q":
		m.cancel = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items())-1 {
			m.cursor++
		}
	case "left", "h", "backspace":
		if m.current != "" {
			m.current = ParentProjectPath(m.current)
			m.cursor = 0
			m.message = ""
			m.reload()
		}
	case "enter":
		items := m.items()
		if len(items) == 0 || m.cursor >= len(items) {
			return m, nil
		}
		item := items[m.cursor]
		switch item.kind {
		case pickerItemStart:
			m.selected = m.current
			return m, tea.Quit
		case pickerItemCreate:
			m.mode = pickerModeInput
			m.input = ""
			m.message = ""
		case pickerItemProject:
			m.current = item.path
			m.cursor = 0
			m.message = ""
			m.reload()
		}
	}
	return m, nil
}

func (m pickerModel) updateInput(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyCtrlC:
		m.cancel = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.mode = pickerModeNavigate
		m.input = ""
		m.message = ""
	case tea.KeyEnter:
		createdPath, err := m.store.CreateProject(m.current, m.input)
		if err != nil {
			m.message = "error: " + err.Error()
			return m, nil
		}
		m.mode = pickerModeNavigate
		m.input = ""
		m.message = "created " + createdPath
		m.reload()
		m.moveCursorToProject(createdPath)
	case tea.KeyBackspace:
		runes := []rune(m.input)
		if len(runes) > 0 {
			m.input = string(runes[:len(runes)-1])
		}
	case tea.KeySpace:
		m.input += " "
	case tea.KeyRunes:
		m.input += string(key.Runes)
	}
	return m, nil
}

func (m *pickerModel) reload() {
	children, err := m.store.ListChildProjects(m.current)
	if err != nil {
		m.children = nil
		m.message = "error: " + err.Error()
		return
	}
	m.children = children
	if m.cursor >= len(m.items()) {
		m.cursor = max(0, len(m.items())-1)
	}
}

func (m *pickerModel) moveCursorToProject(projectPath string) {
	for index, item := range m.items() {
		if item.kind == pickerItemProject && item.path == projectPath {
			m.cursor = index
			return
		}
	}
}

func (m pickerModel) items() []pickerItem {
	items := make([]pickerItem, 0, len(m.children)+2)
	if m.purpose == pickerPurposeStart && m.current != "" {
		items = append(items, pickerItem{
			kind: pickerItemStart,
			text: "start timer on current project",
		})
	}

	createText := "create new project"
	if m.current != "" {
		createText = "create new project under " + m.current
	}
	items = append(items, pickerItem{
		kind: pickerItemCreate,
		text: createText,
	})

	for _, child := range m.children {
		items = append(items, pickerItem{
			kind: pickerItemProject,
			path: child.Path,
			text: BaseProjectName(child.Path),
		})
	}
	return items
}

func (m pickerModel) title() string {
	command := "strata project"
	if m.purpose == pickerPurposeStart {
		command = "strata start"
	}
	if m.current == "" {
		return command + " /"
	}
	return command + " /" + m.current
}
