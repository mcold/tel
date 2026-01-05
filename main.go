package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

func getTelDirPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	telDir := filepath.Join(usr.HomeDir, ".tel")
	if err := os.MkdirAll(telDir, 0755); err != nil {
		return "", err
	}
	return telDir, nil
}

func getSqliteDBPath() (string, error) {
	dir, err := getTelDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tel.db"), nil
}

func initSqliteDB() error {
	dbPath, err := getSqliteDBPath()
	if err != nil {
		return err
	}

	sqliteDB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	ddl := `
	CREATE TABLE IF NOT EXISTS items(
		id      INTEGER PRIMARY KEY AUTOINCREMENT
		, name  TEXT
	);

	CREATE TABLE IF NOT EXISTS config
	(
		id_item INTEGER
		, var TEXT
		, val TEXT
		, PRIMARY KEY (id_item, var)
		, FOREIGN KEY (id_item) REFERENCES items(id)
	);

	CREATE TABLE SQL
	(
		id_item INTEGER
		, query text
		, config text
		, FOREIGN KEY (id_item) REFERENCES items(id)
	);
	`

	_, _ = sqliteDB.Exec(ddl)
	return nil
}

func insertItemIfNotExists(item string) error {
	var count int
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM items WHERE name = ?", item).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = sqliteDB.Exec("INSERT INTO items (name) VALUES (?)", item)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertConfig(item string, row table.Row, columns []table.Column) error {
	for i, col := range columns {
		if i < len(row) {
			varName := strings.ToUpper(col.Title)
			varValue := row[i]
			_, err := sqliteDB.Exec(
				"INSERT OR REPLACE INTO config (id_item, var, val) VALUES (?, ?, ?)",
				item, varName, varValue,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func saveToConfig(item string, row table.Row, columns []table.Column) error {
	if err := insertItemIfNotExists(item); err != nil {
		return err
	}
	return insertConfig(item, row, columns)
}

type databaseType struct {
	*sql.DB
	Path             string
	ConnectionString string
}

var database databaseType
var sqliteDB *sql.DB

func (database *databaseType) Connect() error {
	database.ConnectionString = "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	db, err := sql.Open("pgx", database.ConnectionString)
	if err != nil {
		return err
	}

	if err = db.Ping(); err != nil {
		return err
	}
	database.DB = db
	return nil
}

func getContent() ([]table.Row, []table.Column, error) {
	query := `select oid::text as oid, nspname, nspowner::text as nspowner, nspacl::text as nspacl from pg_namespace`
	rows, err := database.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var result []table.Row
	for rows.Next() {
		values := make([]interface{}, len(cols))
		pointers := make([]interface{}, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, nil, err
		}
		row := make(table.Row, len(cols))
		for i, v := range values {
			switch val := v.(type) {
			case nil:
				row[i] = ""
			case []byte:
				row[i] = string(val)
			case string:
				row[i] = val
			default:
				row[i] = fmt.Sprintf("%v", val)
			}
		}
		result = append(result, row)
	}

	tableCols := make([]table.Column, len(cols))
	for i, col := range cols {
		tableCols[i] = table.Column{Title: strings.ToUpper(col), Width: 20}
	}
	return result, tableCols, nil
}

type model struct {
	table    table.Model
	itemName string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			row := m.table.SelectedRow()
			cols := m.table.Columns()
			if err := saveToConfig(m.itemName, row, cols); err != nil {
				return m, tea.Batch(
					tea.Printf("\nError saving to config: %v\n", err),
				)
			}
			return m, tea.Batch()
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return baseStyle.Render(m.table.View()) + "\n"
}

func main() {
	itemName := flag.String("item", "", "Item name for config")
	flag.Parse()

	if *itemName == "" {
		log.Println("Error: -item flag is required")
		os.Exit(1)
	}

	if err := initSqliteDB(); err != nil {
		log.Println("Error initializing SQLite DB:", err)
		os.Exit(1)
	}

	if err := database.Connect(); err != nil {
		log.Println("Error connecting to database:", err)
		os.Exit(1)
	}
	defer database.Close()

	rows, columns, err := getContent()
	if err != nil {
		log.Println("Error getting content:", err)
		os.Exit(1)
	}

	if len(rows) == 0 || len(columns) == 0 {
		log.Println("Error: no data returned")
		os.Exit(1)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{t, *itemName}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
