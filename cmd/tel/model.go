package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mcold/tel/config"
	"mcold/tel/db"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type Model struct {
	table         table.Model
	textInput     textinput.Model
	itemName      string
	sqlQuery      string
	height        int
	widths        map[string]int
	initialFilter string
	uid           string
	filter        string
	view          string
	dataDir       string
}

func NewModel(t table.Model, ti textinput.Model, itemName, sqlQuery string, height int, widths map[string]int, initialFilter string, uid string, view string, dataDir string) Model {
	return Model{
		table:         t,
		textInput:     ti,
		itemName:      itemName,
		sqlQuery:      sqlQuery,
		height:        height,
		widths:        widths,
		initialFilter: initialFilter,
		uid:           uid,
		filter:        initialFilter,
		view:          view,
		dataDir:       dataDir,
	}
}

func (m *Model) SetTable(t table.Model) {
	m.table = t
}

func ToVerticalView(rows []table.Row, cols []table.Column) ([]table.Row, []table.Column) {
	if len(rows) == 0 {
		return rows, cols
	}

	row := rows[0]
	verticalRows := make([]table.Row, len(cols))
	for i := range cols {
		colTitle := cols[i].Title
		var value string
		if i < len(row) {
			value = row[i]
		}
		verticalRows[i] = table.Row{colTitle, value}
	}

	verticalCols := []table.Column{
		{Title: "column", Width: 30},
		{Title: "val", Width: 50},
	}

	return verticalRows, verticalCols
}

func (m *Model) SelectRowByHash(targetHash string) {
	rows := m.table.Rows()
	for i, row := range rows {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(row, "|"))))
		if hash == targetHash {
			m.table.SetCursor(i)
			break
		}
	}
}

func (m Model) FilterContent(filter string) ([]table.Row, []table.Column, error) {
	filter = strings.TrimSpace(filter)
	filter = strings.TrimPrefix(filter, "WHERE")
	filter = strings.TrimSpace(filter)

	var rows []table.Row
	var cols []table.Column
	var err error

	if filter == "" {
		rows, cols, err = db.GetContent(m.sqlQuery)
	} else {
		wrappedQuery := fmt.Sprintf("SELECT * FROM (%s)", m.sqlQuery)
		filteredQuery := fmt.Sprintf("%s WHERE %s", wrappedQuery, filter)
		rows, cols, err = db.GetContent(filteredQuery)
	}
	if err != nil {
		return nil, nil, err
	}

	for i := range cols {
		colTitle := strings.ToUpper(cols[i].Title)
		if width, ok := m.widths[colTitle]; ok {
			cols[i].Width = width
		} else {
			cols[i].Width = 20
		}
	}

	if m.view == "c" {
		rows, cols = ToVerticalView(rows, cols)
	}

	return rows, cols, nil
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if m.table.Focused() {
				m.table.Blur()
				m.textInput.Focus()
			} else {
				m.textInput.Blur()
				m.table.Focus()
			}
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			filter := m.textInput.Value()

			if m.textInput.Focused() {
				rows, cols, err := m.FilterContent(filter)
				if err != nil {
					return m, tea.Batch(
						tea.Printf("\nError filtering: %v\n", err),
					)
				}
				m.table.SetRows(rows)
				m.table.SetColumns(cols)
			}

			row := m.table.SelectedRow()
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(row, "|"))))
			if _, err := config.SaveInstance(m.dataDir, m.itemName, hash, m.uid, filter); err != nil {
				log.Printf("Error saving instance: %v", err)
			}
			return m, tea.Batch()
		}
	}

	m.table, cmd = m.table.Update(msg)
	m.textInput, cmd = m.textInput.Update(msg)

	if m.textInput.Focused() {
		m.filter = m.textInput.Value()
	}

	return m, cmd
}

func (m Model) View() string {
	return baseStyle.Render(m.table.View()) + "\n" + m.textInput.View()
}
