package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mcold/tel/config"
	"mcold/tel/db"
)

func applyColumnWidths(columns []table.Column, widths map[string]int) []table.Column {
	for i := range columns {
		if width, ok := widths[columns[i].Title]; ok {
			columns[i].Width = width
		} else {
			columns[i].Width = 20
		}
	}
	return columns
}

func main() {
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

	itemName := flag.String("item", "", "Item name (required)")
	filter := flag.String("filter", "", "Initial filter for text input")
	uid := flag.String("uid", "", "UID to restore session state")
	viewFlag := flag.String("view", "", "View mode: 'r' or 'c'")
	flag.Parse()

	if *itemName == "" {
		fmt.Fprintln(os.Stderr, "Usage: tel -item <name> [-filter <expr>] [-uid <uid>] [-view r|c]")
		os.Exit(1)
	}

	dataDir := "data"

	resolvedItem, err := config.FindItem(dataDir, *itemName)
	if err != nil {
		log.Printf("ERROR: FindItem failed: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to find item: %v\n", err)
		os.Exit(1)
	}
	log.Printf("Resolved item: %s -> %s", *itemName, resolvedItem)

	metaCfg, err := config.LoadItemConfig(dataDir, resolvedItem)
	if err != nil {
		log.Printf("ERROR: LoadItemConfig failed: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to load item config: %v\n", err)
		os.Exit(1)
	}
	log.Printf("Loaded config: driver=%s, height=%d, view=%s",
		metaCfg.Connection.Driver, metaCfg.Layout.Height, metaCfg.Layout.View)

	sqlQuery, err := config.LoadQuery(dataDir, resolvedItem)
	if err != nil {
		log.Printf("ERROR: LoadQuery failed: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to load query: %v\n", err)
		os.Exit(1)
	}
	log.Printf("Loaded query: %s", sqlQuery)

	dsn := config.ResolveDSN(&metaCfg.Connection)
	log.Printf("DSN: %s", dsn)

	if err := db.Connect(metaCfg.Connection.Driver, dsn); err != nil {
		log.Printf("ERROR: db.Connect failed: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	log.Println("Database connected")
	defer db.Close()

	rows, columns, err := db.GetContent(sqlQuery)
	if err != nil {
		log.Printf("ERROR: db.GetContent failed: %v", err)
		os.Exit(1)
	}
	log.Printf("Retrieved %d rows, %d columns", len(rows), len(columns))

	if len(rows) == 0 || len(columns) == 0 {
		log.Println("ERROR: No rows or columns retrieved")
		os.Exit(1)
	}

	columns = applyColumnWidths(columns, metaCfg.Layout.Widths)

	tblHeight := metaCfg.Layout.Height
	if len(rows) < tblHeight {
		tblHeight = len(rows)
	}
	tblHeight++

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

	if *filter == "" && *uid != "" {
		loadedFilter, err := config.GetFilterByUID(dataDir, resolvedItem, *uid)
		if err != nil {
			log.Printf("WARN: GetFilterByUID failed: %v", err)
		} else if loadedFilter != "" {
			*filter = loadedFilter
		}
	}

	if *filter != "" {
		ti.SetValue(*filter)
	}

	view := *viewFlag
	if view == "" {
		view = metaCfg.Layout.View
	}

	m := NewModel(t, ti, resolvedItem, sqlQuery, tblHeight, metaCfg.Layout.Widths, *filter, *uid, view, dataDir)

	if *filter != "" {
		rows, cols, err := m.FilterContent(*filter)
		if err == nil && len(rows) > 0 {
			t.SetRows(rows)
			t.SetColumns(cols)
			m.SetTable(t)
		}
	} else if view == "c" {
		rows, cols := ToVerticalView(rows, columns)
		t.SetRows(rows)
		t.SetColumns(cols)
		m.SetTable(t)
	}

	if *uid != "" {
		hash, err := config.GetHashByUID(dataDir, resolvedItem, *uid)
		if err == nil {
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
