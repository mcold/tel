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
		id_item TEXT
		, var TEXT
		, val TEXT
		, PRIMARY KEY (id_item, var)
	);
	`

	_, err = sqliteDB.Exec(ddl)
	return err
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
		log.Printf("Item '%s' inserted", item)
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

func getContent() ([]table.Row, error) {
	query := `select oid::text as oid, nspname, nspowner::text as nspowner, nspacl::text as nspacl from pg_namespace`
	rows, err := database.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []table.Row
	for rows.Next() {
		var oid, nspowner string
		var nspname, nspacl sql.NullString
		if err := rows.Scan(&oid, &nspname, &nspowner, &nspacl); err != nil {
			return nil, err
		}
		nspaclStr := ""
		if nspacl.Valid {
			nspaclStr = nspacl.String
		}
		result = append(result, table.Row{oid, nspname.String, nspowner, nspaclStr})
	}
	return result, nil
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
			return m, tea.Batch(
				tea.Printf("\n✓ Saved to config for item: %s\n", m.itemName),
			)
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

	rows, err := getContent()
	if err != nil {
		log.Println("Error getting content:", err)
		os.Exit(1)
	}

	columns := []table.Column{
		{Title: "OID", Width: 10},
		{Title: "Nspname", Width: 20},
		{Title: "Nspowner", Width: 10},
		{Title: "Nspacl", Width: 20},
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
