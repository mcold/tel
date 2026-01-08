package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/marcboeker/go-duckdb/v2"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	Path             string
	ConnectionString string
}

var db DB

func Connect(driver string, connectionString string) error {
	sqlDB, err := sql.Open(driver, connectionString)
	if err != nil {
		return err
	}

	if err = sqlDB.Ping(); err != nil {
		return err
	}

	if driver == "duckdb" {
		if err := executeDuckDBRC(sqlDB); err != nil {
			return err
		}
	}

	db.DB = sqlDB
	db.ConnectionString = connectionString
	return nil
}

func executeDuckDBRC(sqlDB *sql.DB) error {
	rcPath := filepath.Join(os.Getenv("HOME"), ".duckdbrc")
	data, err := os.ReadFile(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_, err = sqlDB.Exec(string(data))
	return err
}

func Close() error {
	return db.Close()
}

func GetContent(sqlQuery string) ([]table.Row, []table.Column, error) {
	rows, err := db.Query(sqlQuery)
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
