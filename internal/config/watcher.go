package config

import (
	"log"
	"os"
	"sync"
	"time"
)

type ReloadableConfig struct {
	mu       sync.RWMutex
	config   *YAMLConfig
	watcher  *fileWatcher
	onChange func(*YAMLConfig)
}

type fileWatcher struct {
	path    string
	modTime time.Time
}

func NewReloadableConfig(path string, onChange func(*YAMLConfig)) (*ReloadableConfig, error) {
	cfg, err := LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	rc := &ReloadableConfig{
		config:   cfg,
		onChange: onChange,
	}

	if err := rc.startWatcher(path); err != nil {
		log.Printf("[CONFIG] Hot reload disabled: %v", err)
	}

	return rc, nil
}

func (rc *ReloadableConfig) startWatcher(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}

	rc.watcher = &fileWatcher{
		path:    path,
		modTime: stat.ModTime(),
	}

	go rc.watchLoop()

	log.Printf("[CONFIG] Hot reload enabled for: %s", path)
	return nil
}

func (rc *ReloadableConfig) watchLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if rc.watcher == nil {
			continue
		}

		stat, err := os.Stat(rc.watcher.path)
		if err != nil {
			continue
		}

		if stat.ModTime().After(rc.watcher.modTime) {
			rc.watcher.modTime = stat.ModTime()
			rc.reload()
		}
	}
}

func (rc *ReloadableConfig) reload() {
	log.Printf("[CONFIG] Reloading configuration...")

	newCfg, err := LoadFromFile(rc.watcher.path)
	if err != nil {
		log.Printf("[CONFIG] Failed to reload config: %v", err)
		return
	}

	oldCfg := rc.GetConfig()
	if oldCfg != nil {
		newCfg.Proxy = oldCfg.Proxy
		newCfg.Auth = oldCfg.Auth
	}

	rc.mu.Lock()
	rc.config = newCfg
	if rc.onChange != nil {
		rc.onChange(newCfg)
	}
	rc.mu.Unlock()

	log.Printf("[CONFIG] Configuration reloaded successfully")
}

func mergeConfigs(oldCfg, newCfg *YAMLConfig) {
	newCfg.Proxy = oldCfg.Proxy
	newCfg.Auth = oldCfg.Auth
}

func (rc *ReloadableConfig) Get() YAMLConfig {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.config == nil {
		return YAMLConfig{}
	}
	return *rc.config
}

func (rc *ReloadableConfig) GetConfig() *YAMLConfig {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.config
}

func (rc *ReloadableConfig) Stop() {
	rc.watcher = nil
}

type WatcherFunc func(*YAMLConfig)

func WatchConfig(path string, onChange WatcherFunc) (*ReloadableConfig, error) {
	return NewReloadableConfig(path, onChange)
}
