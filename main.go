package main

import (
	"database/sql"
	"encoding/json"
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
	CREATE TABLE IF NOT EXISTS dbs(
		id      INTEGER PRIMARY KEY AUTOINCREMENT
		, driver STRING NOT NULL
		, name	STRING UNIQUE
		, connect TEXT
		, comment TEXT
	);

	CREATE TABLE IF NOT EXISTS items(
		id      INTEGER PRIMARY KEY AUTOINCREMENT
		, id_db	INTEGER
		, name  TEXT
		, FOREIGN KEY (id_db) REFERENCES dbs(id)
	);

	CREATE TABLE IF NOT EXISTS config
	(
		id_item INTEGER
		, var STRING UNIQUE
		, val TEXT
		, PRIMARY KEY (id_item, var)
		, FOREIGN KEY (id_item) REFERENCES items(id)
	);

	CREATE TABLE queries
	(
		id_item INTEGER
		, name STRING UNIQUE
		, query TEXT
		, config TEXT
		, FOREIGN KEY (id_item) REFERENCES items(id)
	);
	`

	_, _ = sqliteDB.Exec(ddl)
	return nil
}

func insertItemIfNotExists(item string, idDB int) error {
	var count int
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM items WHERE name = ?", item).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = sqliteDB.Exec("INSERT INTO items (name, id_db) VALUES (?, ?)", item, idDB)
		if err != nil {
			return err
		}
	}
	return nil
}

func getConnectionString(dbName string) (string, error) {
	var connect string
	err := sqliteDB.QueryRow("SELECT connect FROM dbs WHERE name = ?", dbName).Scan(&connect)
	if err != nil {
		return "", err
	}
	return connect, nil
}

func getDBID(dbName string) (int, error) {
	var id int
	err := sqliteDB.QueryRow("SELECT id FROM dbs WHERE name = ?", dbName).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func getDBDriver(dbName string) (string, error) {
	var driver string
	err := sqliteDB.QueryRow("SELECT driver FROM dbs WHERE name = ?", dbName).Scan(&driver)
	if err != nil {
		return "", err
	}
	return driver, nil
}

func getDBDriverByID(idDB int) (string, error) {
	var driver string
	err := sqliteDB.QueryRow("SELECT driver FROM dbs WHERE id = ?", idDB).Scan(&driver)
	if err != nil {
		return "", err
	}
	return driver, nil
}

func getQueryFromDB(sqlName string) (string, error) {
	var query string
	err := sqliteDB.QueryRow("SELECT query FROM queries WHERE name = ?", sqlName).Scan(&query)
	if err != nil {
		return "", err
	}
	return query, nil
}

func getItemID(itemName string) (int, error) {
	var id int
	err := sqliteDB.QueryRow("SELECT id FROM items WHERE name = ?", itemName).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func getDBIDFromItem(itemID int) (int, error) {
	var idDB int
	err := sqliteDB.QueryRow("SELECT id_db FROM items WHERE id = ?", itemID).Scan(&idDB)
	if err != nil {
		return 0, err
	}
	return idDB, nil
}

func getConnectionStringByItem(itemName string) (string, error) {
	itemID, err := getItemID(itemName)
	if err != nil {
		return "", err
	}
	idDB, err := getDBIDFromItem(itemID)
	if err != nil {
		return "", err
	}
	return getConnectionStringByID(idDB)
}

func getConnectionStringByID(idDB int) (string, error) {
	var connect string
	err := sqliteDB.QueryRow("SELECT connect FROM dbs WHERE id = ?", idDB).Scan(&connect)
	if err != nil {
		return "", err
	}
	return connect, nil
}

type QueryConfig struct {
	Widths map[string]int `json:"widths"`
}

func getQueryConfig(sqlName string) (map[string]int, error) {
	var configJSON sql.NullString
	err := sqliteDB.QueryRow("SELECT config FROM queries WHERE name = ?", sqlName).Scan(&configJSON)
	if err != nil {
		return nil, err
	}

	if !configJSON.Valid || configJSON.String == "" {
		return make(map[string]int), nil
	}

	var widths map[string]int
	err = json.Unmarshal([]byte(configJSON.String), &widths)
	if err != nil {
		return nil, err
	}
	return widths, nil
}

func applyColumnWidths(columns []table.Column, widths map[string]int) []table.Column {
	for i := range columns {
		fieldName := strings.ToUpper(columns[i].Title)
		if width, ok := widths[fieldName]; ok {
			columns[i].Width = width
		} else {
			columns[i].Width = 20
		}
	}
	return columns
}

func insertConfig(idItem int, row table.Row, columns []table.Column) error {
	for i, col := range columns {
		if i < len(row) {
			varName := strings.ToUpper(col.Title)
			varValue := row[i]
			_, err := sqliteDB.Exec(
				"INSERT OR REPLACE INTO config (id_item, var, val) VALUES (?, ?, ?)",
				idItem, varName, varValue,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func saveToConfig(itemName string, idDB int, row table.Row, columns []table.Column) error {
	if err := insertItemIfNotExists(itemName, idDB); err != nil {
		return err
	}
	idItem, err := getItemID(itemName)
	if err != nil {
		return err
	}
	return insertConfig(idItem, row, columns)
}

type databaseType struct {
	*sql.DB
	Path             string
	ConnectionString string
}

var database databaseType
var sqliteDB *sql.DB

func (database *databaseType) Connect(driver string, connectionString string) error {
	db, err := sql.Open(driver, connectionString)
	if err != nil {
		return err
	}

	if err = db.Ping(); err != nil {
		return err
	}
	database.DB = db
	database.ConnectionString = connectionString
	return nil
}

func getContent(sqlQuery string) ([]table.Row, []table.Column, error) {
	query := sqlQuery
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
	sqlQuery string
	idDB     int
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
			if err := saveToConfig(m.itemName, m.idDB, row, cols); err != nil {
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
	sqlName := flag.String("sql", "", "SQL query name in queries table")
	dbName := flag.String("db", "", "Database name in dbs table")
	flag.Parse()

	if *itemName == "" {
		log.Println("Error: -item flag is required")
		os.Exit(1)
	}

	if *sqlName == "" {
		log.Println("Error: -sql flag is required")
		os.Exit(1)
	}

	if *dbName == "" {
		log.Println("Error: -db flag is required")
		os.Exit(1)
	}

	if err := initSqliteDB(); err != nil {
		log.Println("Error initializing SQLite DB:", err)
		os.Exit(1)
	}

	idDB, err := getDBID(*dbName)
	if err != nil {
		log.Println("Error getting DB ID:", err)
		os.Exit(1)
	}

	driver, err := getDBDriverByID(idDB)
	if err != nil {
		log.Println("Error getting DB driver:", err)
		os.Exit(1)
	}

	connectionString, err := getConnectionStringByID(idDB)
	if err != nil {
		log.Println("Error getting connection string:", err)
		os.Exit(1)
	}

	sqlQuery, err := getQueryFromDB(*sqlName)
	if err != nil {
		log.Println("Error getting query from DB:", err)
		os.Exit(1)
	}

	queryConfig, err := getQueryConfig(*sqlName)
	if err != nil {
		log.Println("Error getting query config:", err)
		os.Exit(1)
	}

	if err := database.Connect(driver, connectionString); err != nil {
		log.Println("Error connecting to database:", err)
		os.Exit(1)
	}
	defer database.Close()

	rows, columns, err := getContent(sqlQuery)
	if err != nil {
		log.Println("Error getting content:", err)
		os.Exit(1)
	}

	if len(rows) == 0 || len(columns) == 0 {
		log.Println("Error: no data returned")
		os.Exit(1)
	}

	columns = applyColumnWidths(columns, queryConfig)

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

	m := model{t, *itemName, sqlQuery, idDB}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
