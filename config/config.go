package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

type InstanceData struct {
	Hash   string `toml:"hash"`
	Filter string `toml:"filter"`
}

type InstanceFile struct {
	Instances map[string]InstanceData `toml:"instances"`
}

var (
	instanceCache = make(map[string]*InstanceFile)
	cacheMu       sync.RWMutex
)

func getInstancePath(dataDir, itemName string) string {
	return filepath.Join(dataDir, itemName, "instance.toml")
}

func loadInstanceFile(dataDir, itemName string) (*InstanceFile, error) {
	cacheKey := filepath.Join(dataDir, itemName)
	cacheMu.RLock()
	if cached, ok := instanceCache[cacheKey]; ok {
		cacheMu.RUnlock()
		return cached, nil
	}
	cacheMu.RUnlock()

	path := getInstancePath(dataDir, itemName)
	inst := &InstanceFile{
		Instances: make(map[string]InstanceData),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return inst, nil
		}
		return nil, fmt.Errorf("reading instance file: %w", err)
	}

	if err := toml.Unmarshal(data, inst); err != nil {
		return nil, fmt.Errorf("parsing instance file: %w", err)
	}

	cacheMu.Lock()
	instanceCache[cacheKey] = inst
	cacheMu.Unlock()

	return inst, nil
}

func saveInstanceFile(dataDir, itemName string, inst *InstanceFile) error {
	path := getInstancePath(dataDir, itemName)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating instance directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating instance file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(inst); err != nil {
		return fmt.Errorf("encoding instance file: %w", err)
	}

	cacheKey := filepath.Join(dataDir, itemName)
	cacheMu.Lock()
	instanceCache[cacheKey] = inst
	cacheMu.Unlock()

	return nil
}

func generateUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func SaveInstance(dataDir, itemName string, hash string, providedUID string, filter string) (string, error) {
	inst, err := loadInstanceFile(dataDir, itemName)
	if err != nil {
		return "", err
	}

	uid := providedUID
	if uid == "" {
		uid = generateUID()
	}

	inst.Instances[uid] = InstanceData{
		Hash:   hash,
		Filter: filter,
	}

	if err := saveInstanceFile(dataDir, itemName, inst); err != nil {
		return "", err
	}

	return uid, nil
}

func GetHashByUID(dataDir, itemName string, uid string) (string, error) {
	inst, err := loadInstanceFile(dataDir, itemName)
	if err != nil {
		return "", err
	}

	if data, ok := inst.Instances[uid]; ok {
		return data.Hash, nil
	}

	return "", fmt.Errorf("instance not found for uid %s", uid)
}

func GetFilterByUID(dataDir, itemName string, uid string) (string, error) {
	inst, err := loadInstanceFile(dataDir, itemName)
	if err != nil {
		return "", err
	}

	if data, ok := inst.Instances[uid]; ok {
		return data.Filter, nil
	}

	return "", fmt.Errorf("instance not found for uid %s", uid)
}
