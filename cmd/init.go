package cmd

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/filesystem"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

//go:embed default_schema.json
var defaultSchema []byte

//go:embed default_gitignore
var gitignoreContents []byte

const createNewDBOption = "<Create a new database>"

func InitCommand(c *cli.Context) error {
	dir := c.String("dir")
	useYAML := c.Bool("yaml")

	exists, err := filesystem.FileExists(dir)
	if err != nil {
		return err
	}
	if exists && !c.Bool("force") {
		return fmt.Errorf("Directory `%s` already exists, so I am not overwriting it. Use -f if you are sure.", dir)
	}
	if exists {
		err := os.RemoveAll(dir)
		if err != nil {
			return fmt.Errorf("removing old `%s` directory: %w", dir, err)
		}
	}

	workspaceID := c.String("workspaceid")
	if workspaceID == "" {
		existingWorkspaces, err := getWorkspaces(c)
		if err != nil {
			return err
		}
		switch len(existingWorkspaces) {
		case 0:
			return errors.New("no workspaces found, please create one first")
		case 1:
			workspaceID = existingWorkspaces[0]
			fmt.Printf("You only have a workspace, using it by default: %s\n", workspaceID)
		default:
			prompt := &survey.Select{
				Message: "Select the workspace for the database: ",
				Options: existingWorkspaces,
				Default: existingWorkspaces[0],
			}
			err = survey.AskOne(prompt, &workspaceID, nil)
			if err != nil {
				return err
			}
		}
	}

	dbname := c.String("dbname")
	var pull bool
	if dbname == "" {
		existingDBs, err := getDBs(c, workspaceID)
		if err != nil {
			return err
		}

		currentDir, err := getDirectoryName()
		if len(existingDBs) > 0 {
			defDBName := createNewDBOption
			if err == nil && stringInSlice(currentDir, existingDBs) {
				defDBName = currentDir
			}
			prompt := &survey.Select{
				Message: "Do you want to pull from an existing database?",
				Default: defDBName,
				Options: append([]string{createNewDBOption}, existingDBs...),
			}
			err = survey.AskOne(prompt, &dbname)
			if err != nil {
				return err
			}
		}

		if len(existingDBs) == 0 || dbname == createNewDBOption {
			prompt := &survey.Input{
				Message: "New database name:",
				Default: currentDir,
			}
			err = survey.AskOne(prompt, &dbname, survey.WithValidator(survey.Required))
			if err != nil {
				return err
			}

			// TODO move this as a survey validator
			if len(dbname) == 0 || !spec.IsValidIdentifier(dbname) {
				return fmt.Errorf("Invalid dbname identifier. Identifiers must begin with a letter or number and can include `-`, `_`, `~`.")
			}
		} else {
			// existing db selected, just pull it
			pull = true
		}
	}

	err = ensureDirectory(dir)
	if err != nil {
		return fmt.Errorf("directory: %w", err)
	}

	err = writeGitIgnoreFile(dir)
	if err != nil {
		return fmt.Errorf("creating .gitignore file: %w", err)
	}

	schemaFileFormat := SettingsJSON
	if useYAML {
		schemaFileFormat = SettingsYAML
	}
	err = writeSettings(dir, SettingsFile{
		SchemaFileFormat: schemaFileFormat,
		DBName:           dbname,
		WorkspaceID:      workspaceID,
	})
	if err != nil {
		return err
	}

	schemaFile := path.Join(dir, "schema.json")
	if pull {
		// Pull existing schema
		fmt.Println("Pulling database schema...")
		err = PullCommand(c)
		if err != nil {
			return err
		}
	} else {
		// Create a new schema file
		toWrite := defaultSchema
		if useYAML {
			schemaFile = path.Join(dir, "schema.yaml")

			var schema spec.Schema
			err := json.Unmarshal(defaultSchema, &schema)
			if err != nil {
				return err
			}

			toWrite, err = yaml.Marshal(schema)
			if err != nil {
				return err
			}
		}

		err = os.WriteFile(schemaFile, toWrite, 0644)
		if err != nil {
			return fmt.Errorf("Error writing file %s: %w", schemaFile, err)
		}
	}

	err = InstallCodegen(dir)
	if err != nil {
		return err
	}

	err = RunHook(dir, "build")
	if err != nil {
		return err
	}

	fmt.Printf("Init done. Edit %s to get started.\n", schemaFile)

	return nil
}

func ensureDirectory(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

func writeGitIgnoreFile(dir string) error {
	gitIgnoreFile := path.Join(dir, ".gitignore")
	exists, err := filesystem.FileExists(gitIgnoreFile)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return os.WriteFile(gitIgnoreFile, gitignoreContents, 0644)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
