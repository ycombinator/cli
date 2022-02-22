package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/xata/cli/client"
	"github.com/xata/cli/client/spec"
	"github.com/xata/cli/config"
	"github.com/xata/cli/filesystem"

	"github.com/tidwall/pretty"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

const (
	backupSuffix  = ".bak"
	schemaVersion = "1.0"
)

func PullCommand(c *cli.Context) error {
	dir := c.String("dir")

	settings, err := ReadSettings(dir)
	if err != nil {
		return err
	}

	schemaFile := path.Join(dir, "schema.json")
	if settings.SchemaFileFormat == SettingsYAML {
		schemaFile = path.Join(dir, "schema.yaml")
	}

	dbName, _, branch, err := getDBNameAndBranch(c)
	if err != nil {
		return err
	}

	workspaceID, err := getWorkspaceID(c)
	if err != nil {
		return err
	}

	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	client, err := client.NewXataClientWithResponses(apiKey, workspaceID)
	if err != nil {
		return err
	}

	dbbranch := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, branch))
	resp, err := client.GetBranchDetailsWithResponse(c.Context, dbbranch)
	if err != nil {
		return err
	}
	err = checkBranchDetails(resp)
	if err != nil {
		return err
	}

	backupCreated := false
	exists, err := filesystem.FileExists(schemaFile)
	if err != nil {
		return err
	}
	if exists {
		err := copyFile(schemaFile, schemaFile+backupSuffix)
		if err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
		backupCreated = true
	}
	defer func() {
		if err != nil && backupCreated {
			os.Rename(schemaFile+backupSuffix, schemaFile)
		}
	}()

	baseBranch := resp.JSON200
	version := schemaVersion
	baseBranch.Schema.FormatVersion = version
	var file []byte
	if settings.SchemaFileFormat == SettingsYAML {
		file, err = yaml.Marshal(baseBranch.Schema)
		if err != nil {
			return err
		}
	} else {
		file, err = json.MarshalIndent(baseBranch.Schema, "", " ")
		if err != nil {
			return err
		}
		file = pretty.Pretty(file)
	}

	err = ioutil.WriteFile(schemaFile, file, 0644)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func copyFile(sourceFile, destinationFile string) (err error) {
	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", sourceFile, err)
	}

	err = ioutil.WriteFile(destinationFile, input, 0644)
	if err != nil {
		return fmt.Errorf("creating %s: %w", destinationFile, err)
	}
	return nil
}

func checkBranchDetails(branch *spec.GetBranchDetailsResponse) error {
	if branch.JSON401 != nil {
		return ErrorUnauthorized{message: branch.JSON401.Message}
	}
	if branch.JSON400 != nil {
		return fmt.Errorf("Error getting branch details: %s", branch.JSON400.Message)
	}
	if branch.JSON404 != nil {
		return fmt.Errorf("Error getting branch details: %s", branch.JSON404.Message)
	}

	if branch.StatusCode() != http.StatusOK {
		return fmt.Errorf("Error getting branch details: %s", branch.Status())
	}
	if branch.JSON200 == nil {
		return fmt.Errorf("Error getting branch details: 200 OK unexpected response body")
	}
	return nil
}
