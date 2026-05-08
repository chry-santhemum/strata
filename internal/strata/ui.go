package strata

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var ErrPickerCancelled = errors.New("selection cancelled")
var ErrFocusPlanAborted = errors.New("focus plan aborted")

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

func RunFocusPlanPrompt(store *Store) error {
	state, err := store.LoadState()
	if err != nil {
		return err
	}
	if state == nil {
		return errors.New("no active timer")
	}

	model := newFocusPlanModel(store, state.Plan)
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return err
	}
	model, ok := finalModel.(focusPlanModel)
	if !ok {
		return nil
	}
	if model.aborted {
		if err := store.ClearState(); err != nil {
			return err
		}
		return ErrFocusPlanAborted
	}
	return nil
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

type focusPlanModel struct {
	store   *Store
	index   int
	values  []string
	cursors []int
	message string
	aborted bool
}

func newFocusPlanModel(store *Store, plan *FocusPlan) focusPlanModel {
	answers := focusPlanAnswers(plan)
	return focusPlanModel{
		store:   store,
		values:  append([]string(nil), answers...),
		cursors: []int{len([]rune(answers[0])), len([]rune(answers[1])), len([]rune(answers[2]))},
	}
}

func (m focusPlanModel) Init() tea.Cmd {
	return nil
}

func (m focusPlanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.aborted = true
		return m, tea.Quit
	case tea.KeyTab:
		m = m.save()
		if m.index == len(focusPlanQuestions)-1 {
			return m, tea.Quit
		}
		m.index++
	case tea.KeyShiftTab:
		m = m.save()
		if m.index > 0 {
			m.index--
		}
	case tea.KeyEnter:
		m.insert("\n")
		m = m.save()
	case tea.KeyBackspace:
		m.backspace()
		m = m.save()
	case tea.KeyDelete:
		m.delete()
		m = m.save()
	case tea.KeyLeft:
		if m.cursors[m.index] > 0 {
			m.cursors[m.index]--
		}
	case tea.KeyRight:
		if m.cursors[m.index] < len([]rune(m.values[m.index])) {
			m.cursors[m.index]++
		}
	case tea.KeyUp:
		m.cursors[m.index] = moveCursorVertically(m.values[m.index], m.cursors[m.index], -1)
	case tea.KeyDown:
		m.cursors[m.index] = moveCursorVertically(m.values[m.index], m.cursors[m.index], 1)
	case tea.KeyRunes:
		m.insert(string(key.Runes))
		m = m.save()
	case tea.KeySpace:
		m.insert(" ")
		m = m.save()
	}
	return m, nil
}

func (m focusPlanModel) View() string {
	var builder strings.Builder
	builder.WriteString("Some questions to help you plan:\n\n")
	fmt.Fprintf(&builder, "%d/%d\n", m.index+1, len(focusPlanQuestions))
	builder.WriteString(focusPlanQuestions[m.index])
	builder.WriteString("\n\n")
	builder.WriteString(renderEditableText(m.values[m.index], m.cursors[m.index]))
	if m.message != "" {
		fmt.Fprintf(&builder, "\n%s\n", m.message)
	}
	if m.index == len(focusPlanQuestions)-1 {
		builder.WriteString("\nenter newline | tab finish | shift+tab previous | esc abort timer\n")
	} else {
		builder.WriteString("\nenter newline | tab next | shift+tab previous | esc abort timer\n")
	}
	return builder.String()
}

func (m focusPlanModel) save() focusPlanModel {
	plan := focusPlanFromAnswers(m.values)
	if err := m.store.SaveCurrentPlan(plan); err != nil {
		m.message = "error: " + err.Error()
	} else {
		m.message = ""
	}
	return m
}

func (m *focusPlanModel) insert(text string) {
	runes := []rune(m.values[m.index])
	cursor := m.cursors[m.index]
	inserted := []rune(text)
	runes = append(runes[:cursor], append(inserted, runes[cursor:]...)...)
	m.values[m.index] = string(runes)
	m.cursors[m.index] += len(inserted)
}

func (m *focusPlanModel) backspace() {
	cursor := m.cursors[m.index]
	if cursor == 0 {
		return
	}
	runes := []rune(m.values[m.index])
	runes = append(runes[:cursor-1], runes[cursor:]...)
	m.values[m.index] = string(runes)
	m.cursors[m.index]--
}

func (m *focusPlanModel) delete() {
	cursor := m.cursors[m.index]
	runes := []rune(m.values[m.index])
	if cursor >= len(runes) {
		return
	}
	runes = append(runes[:cursor], runes[cursor+1:]...)
	m.values[m.index] = string(runes)
}

func renderEditableText(value string, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	withCursor := string(runes[:cursor]) + "|" + string(runes[cursor:])
	lines := strings.Split(withCursor, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func moveCursorVertically(value string, cursor, delta int) int {
	lines, lineIndex, column := cursorLineColumn(value, cursor)
	nextLine := lineIndex + delta
	if nextLine < 0 || nextLine >= len(lines) {
		return cursor
	}
	if column > len([]rune(lines[nextLine])) {
		column = len([]rune(lines[nextLine]))
	}

	nextCursor := 0
	for i := 0; i < nextLine; i++ {
		nextCursor += len([]rune(lines[i])) + 1
	}
	return nextCursor + column
}

func cursorLineColumn(value string, cursor int) ([]string, int, int) {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	lines := strings.Split(value, "\n")
	lineIndex := 0
	lineStart := 0
	for i := 0; i < cursor && i < len(runes); i++ {
		if runes[i] == '\n' {
			lineIndex++
			lineStart = i + 1
		}
	}
	return lines, lineIndex, cursor - lineStart
}
