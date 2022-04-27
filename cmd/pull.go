package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"
	"github.com/xataio/cli/filesystem"

	"github.com/AlecAivazis/survey/v2"
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

	client, err := client.NewXataClient(apiKey, workspaceID)
	if err != nil {
		return err
	}
	clientWithResponses := &spec.ClientWithResponses{ClientInterface: client}

	dbBranchName := fmt.Sprintf("%s:%s", dbName, branch)
	dbbranch := spec.DBBranchNameParam(dbBranchName)
	resp, err := clientWithResponses.GetBranchDetailsWithResponse(c.Context, dbbranch)
	if err != nil {
		return err
	}
	if resp.JSON404 != nil {
		if err := errorIfNotInteractive(c, "a new branch name"); err != nil {
			fmt.Println("Branch", dbBranchName, "doesn't exist anymore")
			return err
		}
		difBranch, err := promptUserForADifferentBranch(c, client, dbName, branch)
		if err != nil {
			return err
		}
		dbbranch := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, difBranch))
		resp, err = clientWithResponses.GetBranchDetailsWithResponse(c.Context, dbbranch)
		if err != nil {
			return err
		}
		branch = difBranch
	}
	err = checkBranchDetails(resp)
	if err != nil {
		return err
	}

	fmt.Printf("Pulling schema from branch [%s:%s]\n", dbName, branch)

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
		fmt.Printf("Your existing schema file has been backed up to %s\n", schemaFile+backupSuffix)
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

	err = RunHook(dir, "build")
	if err != nil {
		return err
	}

	fmt.Printf("Pulled schema written to %s\n", schemaFile)
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

func promptUserForADifferentBranch(c *cli.Context, client *spec.Client, dbName, branch string) (string, error) {
	var yes bool
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Branch [%s:%s] does not exist. Would you like to pull the schema from another branch?", dbName, branch),
		Default: true,
	}
	survey.AskOne(prompt, &yes)
	if !yes {
		return "", fmt.Errorf("Ok, exiting.")
	}

	existingBranches, err := getBranches(c, client, dbName)
	if err != nil {
		return "", err
	}

	if len(existingBranches) == 0 {
		return "", fmt.Errorf("No branches found for database [%s]", dbName)
	}

	var fromBranch string
	promptBranch := &survey.Select{
		Message: "From which branch should I pull the schema?",
		Options: existingBranches,
		Default: existingBranches[0],
	}
	survey.AskOne(promptBranch, &fromBranch, nil)

	return fromBranch, nil
}
