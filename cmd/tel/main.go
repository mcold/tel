package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mcold/tel/config"
	"mcold/tel/db"
)

func applyColumnWidths(columns []table.Column, widths map[string]int, aliases map[string]string) []table.Column {
	for i := range columns {
		fieldName := columns[i].Title
		if width, ok := widths[fieldName]; ok {
			columns[i].Width = width
		} else {
			columns[i].Width = 20
		}
	}
	return columns
}

func main() {
	// Initialize log file
	logFilePath := filepath.Join("logs", "tel.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	log.Println("=== Application started ===")

	itemName := flag.String("item", "", "Item name for config")
	sqlName := flag.String("sql", "", "SQL query name in queries table")
	dbName := flag.String("db", "", "Database name in dbs table")
	filter := flag.String("filter", "", "Initial filter for text input")
	args := flag.String("args", "", "JSON with placeholder args in SQL query")
	uid := flag.String("uid", "", "UID to select row by hash from instance table")
	flag.Parse()

	log.Printf("Parsed flags: item=%q, sql=%q, db=%q, filter=%q, uid=%q",
		*itemName, *sqlName, *dbName, *filter, *uid)

	if *itemName == "" {
		log.Println("ERROR: item flag is empty")
		os.Exit(1)
	}
	log.Printf("itemName: %s", *itemName)

	if *sqlName == "" {
		log.Println("ERROR: sql flag is empty")
		os.Exit(1)
	}
	log.Printf("sqlName: %s", *sqlName)

	if *dbName == "" {
		log.Println("ERROR: db flag is empty")
		os.Exit(1)
	}
	log.Printf("dbName: %s", *dbName)

	if err := config.Init(); err != nil {
		log.Printf("ERROR: config.Init failed: %v", err)
		os.Exit(1)
	}
	log.Println("Config initialized successfully")

	idDB, err := config.GetDBID(*dbName)
	if err != nil {
		log.Printf("ERROR: config.GetDBID failed for dbName=%s: %v", *dbName, err)
		os.Exit(1)
	}
	log.Printf("idDB: %d", idDB)

	idItem, err := config.GetItemID(*itemName)
	if err != nil {
		log.Printf("ERROR: config.GetItemID failed for itemName=%s: %v", *itemName, err)
		os.Exit(1)
	}
	log.Printf("idItem: %d", idItem)

	idQuery, err := config.GetQueryID(*sqlName)
	if err != nil {
		log.Printf("ERROR: config.GetQueryID failed for sqlName=%s: %v", *sqlName, err)
		os.Exit(1)
	}
	log.Printf("idQuery: %d", idQuery)

	driver, err := config.GetDBDriverByID(idDB)
	if err != nil {
		log.Printf("ERROR: config.GetDBDriverByID failed for idDB=%d: %v", idDB, err)
		os.Exit(1)
	}
	log.Printf("driver: %s", driver)

	connectionString, err := config.GetConnectionStringByID(idDB)
	if err != nil {
		log.Printf("ERROR: config.GetConnectionStringByID failed for idDB=%d: %v", idDB, err)
		os.Exit(1)
	}
	log.Printf("connectionString: %s", connectionString)

	sqlQuery, err := config.GetQueryFromDB(*sqlName)
	if err != nil {
		log.Printf("ERROR: config.GetQueryFromDB failed for sqlName=%s: %v", *sqlName, err)
		os.Exit(1)
	}
	log.Printf("sqlQuery: %s", sqlQuery)

	if *args != "" {
		file, err := os.Open(*args)
		if err != nil {
			log.Printf("ERROR: can't read file args: %s: %v", *args, err)
			os.Exit(1)
		}
		defer file.Close()

		var data map[string]interface{}
		json.NewDecoder(file).Decode(&data)
		// json.Unmarshal([]byte(jsonStr), &data)
		for k, v := range data {
			valueStr := fmt.Sprintf("%v", v)
			sqlQuery = strings.ReplaceAll(sqlQuery, fmt.Sprintf(":%s", k), valueStr)
		}
		log.Println(sqlQuery)
	}

	widths, aliases, tblHeight, err := config.GetQueryConfig(*sqlName)
	if err != nil {
		log.Printf("ERROR: config.GetQueryConfig failed for sqlName=%s: %v", *sqlName, err)
		os.Exit(1)
	}
	log.Printf("widths: %v, aliases: %v, tblHeight: %d", widths, aliases, tblHeight)

	if err := db.Connect(driver, connectionString); err != nil {
		log.Printf("ERROR: database.Connect failed for driver=%s: %v", driver, err)
		os.Exit(1)
	}
	log.Println("Database connected successfully")
	defer db.Close()

	rows, columns, err := db.GetContent(sqlQuery)
	if err != nil {
		log.Printf("ERROR: database.GetContent failed: %v", err)
		os.Exit(1)
	}
	log.Printf("Retrieved %d rows, %d columns", len(rows), len(columns))

	if len(rows) == 0 || len(columns) == 0 {
		log.Println("ERROR: No rows or columns retrieved from database")
		os.Exit(1)
	}

	columns = applyColumnWidths(columns, widths, aliases)
	log.Printf("Applied column widths: %d columns processed", len(columns))

	if tblHeight == 0 {
		tblHeight = 10
		log.Println("tblHeight was 0, set to default 10")
	}

	if len(rows) < 10 {
		tblHeight = len(rows)
		log.Printf("tblHeight adjusted to %d (rows count)", tblHeight)
	}

	tblHeight = tblHeight + 1
	log.Printf("Final tblHeight: %d", tblHeight)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tblHeight),
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

	ti := textinput.New()
	ti.CharLimit = 500
	ti.Width = 1000

	// Load filter from instance table if uid is provided and filter flag is empty
	if *filter == "" && *uid != "" {
		loadedFilter, err := config.GetFilterByUID(*uid, idQuery)
		if err != nil {
			log.Printf("WARN: GetFilterByUID failed for uid=%s, idQuery=%d: %v", *uid, idQuery, err)
		} else if loadedFilter != "" {
			*filter = loadedFilter
			log.Printf("Filter loaded from instance: %q", *filter)
		}
	}

	if *filter != "" {
		ti.SetValue(*filter)
		log.Printf("Initial filter applied: %q", *filter)
	}

	m := NewModel(t, ti, *itemName, *sqlName, sqlQuery, idDB, idQuery, tblHeight, aliases, *filter, *uid)
	log.Printf("UI Model created: itemName=%s, sqlName=%s, idDB=%d, idQuery=%d, tblHeight=%d, uid=%s",
		*itemName, *sqlName, idDB, idQuery, tblHeight, *uid)

	if *filter != "" {
		rows, cols, err := m.FilterContent(*filter)
		if err == nil && len(rows) > 0 {
			t.SetRows(rows)
			t.SetColumns(cols)
			m.SetTable(t)
			log.Printf("Filter applied: %d rows after filtering", len(rows))
		}
	}

	// Select row by hash if uid flag is provided
	if *uid != "" {
		hash, err := config.GetHashByUID(*uid, idQuery)
		if err != nil {
			log.Printf("WARN: GetHashByUID failed for uid=%s, idQuery=%d: %v", *uid, idQuery, err)
		} else {
			log.Printf("Looking for row with hash=%s", hash)
			m.SelectRowByHash(hash)
		}
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Printf("ERROR: tea.NewProgram.Run failed: %v", err)
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	log.Println("=== Application exited normally ===")
}
