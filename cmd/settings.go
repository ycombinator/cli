package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/xataio/cli/filesystem"

	"github.com/tidwall/pretty"
)

const settingsFilename = "config.json"

const (
	SettingsYAML = "yaml"
	SettingsJSON = "json"
)

type SettingsFile struct {
	SchemaFileFormat string            `json:"schemaFileFormat"`
	DBName           string            `json:"dbName"`
	WorkspaceID      string            `json:"workspaceID"`
	Hooks            map[string]string `json:"hooks"`
}

func writeSettings(dir string, settings SettingsFile) error {
	file, err := json.MarshalIndent(settings, "", " ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(dir, settingsFilename), pretty.Pretty(file), 0644)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

func ReadSettings(dir string) (*SettingsFile, error) {
	exists, err := filesystem.FileExists(dir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("Directory `%s` doesn't exist. Did you run `xata init`?", dir)
	}

	jsonFile, err := os.Open(path.Join(dir, settingsFilename))
	if err != nil {
		return nil, fmt.Errorf("opening file `%s/%s`: %w", dir, settingsFilename, err)
	}
	defer jsonFile.Close()

	bytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("reading file `%s/%s`: %w", dir, settingsFilename, err)
	}

	var settings SettingsFile
	err = json.Unmarshal(bytes, &settings)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling `%s/%s`: %w", dir, settingsFilename, err)
	}

	if settings.SchemaFileFormat != SettingsJSON && settings.SchemaFileFormat != SettingsYAML {
		return nil, fmt.Errorf("the schemaFileFormat setting must be either `json` or `yaml`")
	}
	return &settings, nil
}
