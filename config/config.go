package config

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	_ "modernc.org/sqlite"
)

var sqliteDB *sql.DB

type QueryConfig struct {
	Widths  map[string]int    `json:"widths"`
	Aliases map[string]string `json:"aliases"`
	Height  int               `json:"height"`
}

func GetDBPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	telDir := filepath.Join(usr.HomeDir, ".tel")
	if err := os.MkdirAll(telDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(telDir, "tel.db"), nil
}

func Init() error {
	dbPath, err := GetDBPath()
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
		, height INTEGER DEFAULT 10
		, FOREIGN KEY (id_item) REFERENCES items(id)
	);
	`

	_, _ = sqliteDB.Exec(ddl)
	return nil
}

func GetConnectionString(dbName string) (string, error) {
	var connect string
	err := sqliteDB.QueryRow("SELECT connect FROM dbs WHERE name = ?", dbName).Scan(&connect)
	if err != nil {
		return "", err
	}
	return connect, nil
}

func GetDBID(dbName string) (int, error) {
	var id int
	err := sqliteDB.QueryRow("SELECT id FROM dbs WHERE name = ?", dbName).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func GetDBDriver(dbName string) (string, error) {
	var driver string
	err := sqliteDB.QueryRow("SELECT driver FROM dbs WHERE name = ?", dbName).Scan(&driver)
	if err != nil {
		return "", err
	}
	return driver, nil
}

func GetDBDriverByID(idDB int) (string, error) {
	var driver string
	err := sqliteDB.QueryRow("SELECT driver FROM dbs WHERE id = ?", idDB).Scan(&driver)
	if err != nil {
		return "", err
	}
	return driver, nil
}

func GetQueryFromDB(sqlName string) (string, error) {
	var query string
	err := sqliteDB.QueryRow("SELECT query FROM queries WHERE name = ?", sqlName).Scan(&query)
	if err != nil {
		return "", err
	}
	return query, nil
}

func GetItemID(itemName string) (int, error) {
	var id int
	err := sqliteDB.QueryRow("SELECT id FROM items WHERE name = ?", itemName).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func GetDBIDFromItem(itemID int) (int, error) {
	var idDB int
	err := sqliteDB.QueryRow("SELECT id_db FROM items WHERE id = ?", itemID).Scan(&idDB)
	if err != nil {
		return 0, err
	}
	return idDB, nil
}

func GetConnectionStringByID(idDB int) (string, error) {
	var connect string
	err := sqliteDB.QueryRow("SELECT connect FROM dbs WHERE id = ?", idDB).Scan(&connect)
	if err != nil {
		return "", err
	}
	return connect, nil
}

func GetConnectionStringByItem(itemName string) (string, error) {
	itemID, err := GetItemID(itemName)
	if err != nil {
		return "", err
	}
	idDB, err := GetDBIDFromItem(itemID)
	if err != nil {
		return "", err
	}
	return GetConnectionStringByID(idDB)
}

func GetQueryConfig(sqlName string) (map[string]int, map[string]string, int, error) {
	var configJSON sql.NullString
	var tableHeight int
	err := sqliteDB.QueryRow("SELECT config, COALESCE(height, 10) FROM queries WHERE name = ?", sqlName).Scan(&configJSON, &tableHeight)
	if err != nil {
		return nil, nil, 0, err
	}

	if !configJSON.Valid || configJSON.String == "" {
		return make(map[string]int), make(map[string]string), tableHeight, nil
	}

	var config QueryConfig
	err = json.Unmarshal([]byte(configJSON.String), &config)
	if err != nil {
		return nil, nil, 0, err
	}

	if config.Height == 0 {
		config.Height = tableHeight
	}

	return config.Widths, config.Aliases, config.Height, nil
}

func InsertItemIfNotExists(item string, idDB int) error {
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

func InsertConfig(idItem int, row []string, cols []string, aliases map[string]string) error {
	for i := range cols {
		if i < len(row) {
			colTitle := strings.ToUpper(cols[i])
			if _, ok := aliases[colTitle]; ok {
				varValue := row[i]
				_, err := sqliteDB.Exec(
					"INSERT OR REPLACE INTO config (id_item, var, val) VALUES (?, ?, ?)",
					idItem, aliases[colTitle], varValue,
				)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func SaveToConfig(itemName string, idDB int, row []string, cols []string, aliases map[string]string) error {
	if err := InsertItemIfNotExists(itemName, idDB); err != nil {
		return err
	}
	idItem, err := GetItemID(itemName)
	if err != nil {
		return err
	}

	return InsertConfig(idItem, row, cols, aliases)
}

func SaveConfigFromTable(itemName string, idDB int, row []string, cols []table.Column, aliases map[string]string) error {
	if err := InsertItemIfNotExists(itemName, idDB); err != nil {
		return err
	}
	idItem, err := GetItemID(itemName)
	if err != nil {
		return err
	}

	for i := range cols {
		if i < len(row) {
			colTitle := strings.ToUpper(cols[i].Title)
			if _, ok := aliases[colTitle]; ok {
				varValue := row[i]
				_, err := sqliteDB.Exec(
					"INSERT OR REPLACE INTO config (id_item, var, val) VALUES (?, ?, ?)",
					idItem, aliases[colTitle], varValue,
				)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
