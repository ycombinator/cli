package cmd

import (
	"fmt"

	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
)

func GetBranchesSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "list",
			Usage:  "List branches",
			Action: listBranches,
		},
		{
			Name:   "create",
			Usage:  "Create a branch",
			Action: createBranch,
		},
		{
			Name:   "delete",
			Usage:  "Delete branch",
			Action: deleteBranch,
		},
	}
}

func listBranches(c *cli.Context) error {
	dbName, _, _, err := getDBNameAndBranch(c)
	if err != nil {
		return err
	}

	workspaceID, err := getWorkspaceID(c)
	if err != nil {
		return err
	}

	// check if the DB exists, and what branches it has
	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	client, err := client.NewXataClient(apiKey, workspaceID)
	if err != nil {
		return err
	}

	resp, err := client.GetBranchList(c.Context, spec.DBNameParam(dbName))
	return printResponse(c, resp, err)
}

func createBranch(c *cli.Context) error {
	dbName, _, _, err := getDBNameAndBranch(c)
	if err != nil {
		return err
	}

	branchName := c.Args().Get(0)
	if len(branchName) == 0 {
		return fmt.Errorf("please specify a DB name")
	}
	if !spec.IsValidIdentifier(branchName) {
		return fmt.Errorf("Invalid branch name")
	}

	workspaceID, err := getWorkspaceID(c)
	if err != nil {
		return err
	}

	// check if the DB exists, and what branches it has
	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	client, err := client.NewXataClient(apiKey, workspaceID)
	if err != nil {
		return err
	}

	existingBranches, err := getBranches(c, client, dbName)
	if err != nil {
		return err
	}

	var fromBranch string
	if len(existingBranches) == 1 {
		fromBranch = existingBranches[0]
	} else {
		prompt := &survey.Select{
			Message: "From which branch should I fork the new branch?",
			Options: existingBranches,
			Default: defaultBranchName,
		}
		err = survey.AskOne(prompt, &fromBranch, nil)
		if err != nil {
			return err
		}
	}

	dbBranchName := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, branchName))
	resp, err := client.CreateBranch(c.Context, dbBranchName,
		&spec.CreateBranchParams{
			From: &fromBranch,
		}, spec.CreateBranchJSONRequestBody{})
	return printResponse(c, resp, err)
}

func deleteBranch(c *cli.Context) error {
	dbName, _, _, err := getDBNameAndBranch(c)
	if err != nil {
		return err
	}

	branchName := c.Args().Get(0)
	if len(branchName) == 0 {
		return fmt.Errorf("please specify a DB name")
	}
	if !spec.IsValidIdentifier(branchName) {
		return fmt.Errorf("Invalid branch name")
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

	dbBranchName := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, branchName))
	resp, err := client.DeleteBranch(c.Context, dbBranchName)
	return printResponse(c, resp, err)
}

func getBranches(c *cli.Context, client *spec.Client, dbName string) ([]string, error) {
	cr := spec.ClientWithResponses{ClientInterface: client}
	existingBranches, err := cr.GetBranchListWithResponse(c.Context, spec.DBNameParam(dbName))
	if err != nil {
		return nil, err
	}

	if status := existingBranches.StatusCode(); status > 299 && status != 404 {
		if existingBranches.JSON401 != nil {
			return nil, ErrorUnauthorized{message: existingBranches.JSON401.Message}
		}
		return nil, fmt.Errorf("listing branches: %s", existingBranches.Status())
	}

	branches := []string{}
	if existingBranches.JSON200 != nil {
		for _, branch := range existingBranches.JSON200.Branches {
			branches = append(branches, branch.Name)
		}
	}

	return branches, nil
}
