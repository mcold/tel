package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type ConnectionConfig struct {
	Driver   string `toml:"driver"`
	DSN      string `toml:"dsn,omitempty"`
	Host     string `toml:"host,omitempty"`
	Port     int    `toml:"port,omitempty"`
	User     string `toml:"user,omitempty"`
	Password string `toml:"password,omitempty"`
	DBName   string `toml:"dbname,omitempty"`
}

type LayoutWidths map[string]int

type LayoutConfig struct {
	Height int          `toml:"height"`
	View   string       `toml:"view,omitempty"`
	Widths LayoutWidths `toml:"widths,omitempty"`
}

type MetaConfig struct {
	Connection ConnectionConfig `toml:"connection"`
	Layout     LayoutConfig     `toml:"layout"`
	Variables  map[string]string `toml:"variables,omitempty"`
}

func LoadItemConfig(dataDir, itemName string) (*MetaConfig, error) {
	metaPath := filepath.Join(dataDir, itemName, "meta.toml")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("reading meta.toml for item %q: %w", itemName, err)
	}

	var cfg MetaConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing meta.toml for item %q: %w", itemName, err)
	}

	if cfg.Connection.Driver == "" {
		return nil, fmt.Errorf("meta.toml for item %q: driver is required", itemName)
	}

	if cfg.Layout.Height == 0 {
		cfg.Layout.Height = 10
	}
	if cfg.Layout.View == "" {
		cfg.Layout.View = "r"
	}

	return &cfg, nil
}

func LoadQuery(dataDir, itemName string) (string, error) {
	queryPath := filepath.Join(dataDir, itemName, "query.sql")
	data, err := os.ReadFile(queryPath)
	if err != nil {
		return "", fmt.Errorf("reading query.sql for item %q: %w", itemName, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func ResolveDSN(conn *ConnectionConfig) string {
	switch conn.Driver {
	case "sqlite", "duckdb":
		return conn.DSN
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			conn.Host, conn.Port, conn.User, conn.Password, conn.DBName)
	default:
		return conn.DSN
	}
}

func ListItems(dataDir string) ([]string, error) {
	var items []string
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		metaPath := filepath.Join(path, "meta.toml")
		if _, err := os.Stat(metaPath); err == nil {
			rel, _ := filepath.Rel(dataDir, path)
			items = append(items, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking data directory: %w", err)
	}
	return items, nil
}

func FindItem(dataDir, itemName string) (string, error) {
	items, err := ListItems(dataDir)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item == itemName || filepath.Base(item) == itemName {
			return item, nil
		}
	}
	return "", fmt.Errorf("item %q not found", itemName)
}
