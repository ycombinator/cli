package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/xata/cli/client"
	"github.com/xata/cli/client/spec"
	"github.com/xata/cli/config"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gosimple/slug"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

const defaultBranchName = "main"

func DeployCommand(c *cli.Context) error {
	dir := c.String("dir")
	schema, schemaFile, err := readSchemaFile(dir)
	if err != nil {
		return err
	}

	dbName, repo, branch, err := getDBNameAndBranch(c)
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

	// check if the DB exists, and what branches it has
	existingBranches, err := getBranches(c, client, dbName)
	if err != nil {
		return err
	}

	if len(existingBranches) == 0 {
		// create new database
		if branch == "" {
			branch = defaultBranchName
		}
		req := spec.CreateDatabaseJSONRequestBody{BranchName: &branch}
		resp, err := client.CreateDatabase(c.Context, spec.DBNameParam(dbName), req)
		if err != nil {
			return err
		}
		alreadyExists := resp.StatusCode == 422
		if resp.StatusCode > 299 && !alreadyExists {
			return fmt.Errorf("creating database: %s", resp.Status)
		}
		if !alreadyExists {
			fmt.Printf("Database [%s/%s] created\n", dbName, branch)
		}
	} else {
		mainExists := false
		branchExists := false
		for _, existingBranch := range existingBranches {
			if existingBranch == branch {
				branchExists = true
			}
			if existingBranch == defaultBranchName {
				mainExists = true
			}
		}
		if !branchExists {
			defaultFromBranch := defaultBranchName
			if !mainExists {
				defaultFromBranch = existingBranches[0]
			}
			createBranch, fromBranch, useBranch := promptUserToAskForBranch(dbName, branch, defaultFromBranch, existingBranches)
			if createBranch {
				branchName := spec.BranchName(branch)
				resp, err := client.CreateBranch(c.Context, spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, branch)),
					&spec.CreateBranchParams{
						From: &fromBranch,
					},
					spec.CreateBranchJSONRequestBody{
						Metadata: &spec.BranchMetadata{
							Repository: &repo,
							Branch:     &branchName,
						},
					})
				if err != nil {
					return err
				}
				alreadyExists := resp.StatusCode == 422
				if resp.StatusCode > 299 && !alreadyExists {
					return fmt.Errorf("creating branch: %s", resp.Status)
				}
				if !alreadyExists {
					fmt.Printf("Branch [%s] created starting from the schema of [%s]\n", branch, fromBranch)
				}
			} else {
				branch = useBranch
			}
		}
	}

	dbBranchName := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, branch))
	cr := spec.ClientWithResponses{ClientInterface: client}
	resp, err := cr.GetBranchMigrationPlanWithResponse(c.Context,
		dbBranchName,
		spec.GetBranchMigrationPlanJSONRequestBody(schema))
	if err != nil {
		return fmt.Errorf("Error getting migration plan: %w", err)
	}
	if resp.StatusCode() > 299 {
		return fmt.Errorf("Error getting migration plan: %s", resp.Status())
	}

	plan := resp.JSON200

	// Is plan empty
	if (plan.Migration.NewTables == nil || len(plan.Migration.NewTables.AdditionalProperties) == 0) &&
		plan.Migration.RemovedTables == nil &&
		plan.Migration.TableMigrations == nil {
		fmt.Println("Your schema is up to date.")
		return nil
	}

	fmt.Printf("Migration plan preview:\n\n")
	PrintMigration(plan.Migration, c.Bool("nocolor"))
	fmt.Println()

	var yes bool
	prompt := &survey.Confirm{
		Message: "Apply the above migration?",
		Default: true,
	}
	survey.AskOne(prompt, &yes)

	if yes {
		sha, _ := GitGetLastSHA()
		if sha != "" {
			localChanges, _ := GitHasLocalChanges(schemaFile)
			plan.Migration.LastGitRevision = &sha
			plan.Migration.LocalChanges = localChanges
		}
		now := spec.DateTime(time.Now())
		plan.Migration.CreatedAt = &now
		mResp, err := cr.ExecuteBranchMigrationPlanWithResponse(c.Context,
			dbBranchName,
			spec.ExecuteBranchMigrationPlanJSONRequestBody(*plan))
		if err != nil {
			return fmt.Errorf("Error executing migration: %w", err)
		}
		if mResp.StatusCode() > 299 {
			return fmt.Errorf("Error executing migration: %s", mResp.Status())
		}

		fmt.Println("Done.")
	}

	return nil
}

func readSchemaFile(dir string) (spec.Schema, string, error) {

	settings, err := ReadSettings(dir)
	if err != nil {
		return spec.Schema{}, "", err
	}

	schemaFile := path.Join(dir, "schema.json")
	if settings.SchemaFileFormat == SettingsYAML {
		schemaFile = path.Join(dir, "schema.yaml")
	}

	jsonFile, err := os.Open(schemaFile)
	if err != nil {
		if os.IsNotExist(err) {
			return spec.Schema{}, "", fmt.Errorf("Schema file %s doesn't exist. You can create a sample schema file by running `xata init`", schemaFile)
		}
		return spec.Schema{}, "", fmt.Errorf("opening file %s: %w", schemaFile, err)
	}
	defer jsonFile.Close()

	bytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return spec.Schema{}, "", fmt.Errorf("reading file: %w", err)
	}

	var schema spec.Schema
	if settings.SchemaFileFormat == SettingsYAML {
		err = yaml.Unmarshal(bytes, &schema)
		if err != nil {
			return spec.Schema{}, "", fmt.Errorf("unmarshaling: %w", err)
		}
	} else {
		err = json.Unmarshal(bytes, &schema)
		if err != nil {
			return spec.Schema{}, "", fmt.Errorf("unmarshaling: %w", err)
		}
	}

	return schema, schemaFile, nil
}

func getDBNameAndBranch(c *cli.Context) (dbName, repo, branch string, err error) {
	repo, branch, _ = gitGetRepoAndBranchName()

	settings, err := ReadSettings(c.String("dir"))
	if err != nil {
		return "", "", "", err
	}
	dbName = settings.DBName

	if branch == "" {
		branch = "main"
	}

	// slugify, primarily to ensure no / in the name of the branch
	branch = slug.Make(branch)
	return dbName, repo, branch, nil
}
func getWorkspaceID(c *cli.Context) (string, error) {
	settings, err := ReadSettings(c.String("dir"))
	if err != nil {
		return "", err
	}
	workspaceID := settings.WorkspaceID

	if workspaceID == "" {
		return "", errors.New("workspaceID is missing from settings")
	}

	return workspaceID, nil
}

func promptUserToAskForBranch(dbName, gitBranch, defaultFromBranch string,
	existingBranches []string) (createBranch bool, fromBranch string, useBranch string) {

	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Database [%s] doesn't have a branch [%s]. Would you like to start a new branch?", dbName, gitBranch),
		Default: true,
		Help:    "It is recommended to start a new database branch for each new Git branch.",
	}
	survey.AskOne(prompt, &createBranch)

	if createBranch {
		prompt := &survey.Select{
			Message: "From which branch should I fork the new branch?",
			Options: existingBranches,
			Default: defaultFromBranch,
		}
		survey.AskOne(prompt, &fromBranch, nil)
	} else {
		prompt := &survey.Select{
			Message: "To which existing branch should I deploy?",
			Options: existingBranches,
			Default: defaultFromBranch,
		}
		survey.AskOne(prompt, &useBranch, nil)
	}

	return createBranch, fromBranch, useBranch
}
