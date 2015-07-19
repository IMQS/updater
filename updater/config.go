package updater

import (
	"encoding/json"
	"io/ioutil"
)

// Updater configuration
type Config struct {
	DeployUrl              string  // https://deploy.imqs.co.za/files
	BinDir                 SyncDir // c:/imqsbin
	ConfDir                SyncDir // c:/imqsvar/conf
	LogFile                string  // c:/imqsvar/logs/ImqsUpdater.log
	CheckIntervalSeconds   float64 // 60 * 5
	ServiceStopWaitSeconds float64 // 30
}

// Create a new Config with defaults set
func NewConfig() *Config {
	c := new(Config)
	c.DeployUrl = "https://deploy.imqs.co.za/files"
	c.BinDir.Remote.Path = "imqsbin/stable"
	c.BinDir.LocalPath = "c:/imqsbin"
	c.BinDir.LocalPathNext = "c:/imqsbin_next"
	c.LogFile = "c:/imqsvar/logs/ImqsUpdater.log"
	c.CheckIntervalSeconds = 60 * 5
	c.ServiceStopWaitSeconds = 30
	return c
}

// Read config from JSON file
func (c *Config) LoadFile(filename string) error {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(raw, c)
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) allSyncDirs() []*SyncDir {
	return []*SyncDir{&c.BinDir, &c.ConfDir}
}
