package cmd

import (
	"fmt"

	"github.com/xata/cli/client"
	"github.com/xata/cli/client/spec"
	"github.com/xata/cli/config"

	"github.com/gosimple/slug"
	"github.com/urfave/cli/v2"
)

func GetWorkspacesSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "list",
			Usage:  "List workspaces",
			Action: listWorkspaces,
		},
		{
			Name:   "create",
			Usage:  "Create workspace",
			Action: createWorkspace,
		},
		{
			Name:   "delete",
			Usage:  "Delete workspace",
			Action: deleteWorkspace,
		},
	}
}

func listWorkspaces(c *cli.Context) error {
	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	client, err := client.NewXataClient(apiKey, "")
	if err != nil {
		return err
	}

	resp, err := client.GetWorkspacesList(c.Context)
	return printResponse(c, resp, err)
}

func createWorkspace(c *cli.Context) error {
	workspaceName := c.Args().Get(0)
	if len(workspaceName) == 0 {
		return fmt.Errorf("please specify a workspace name")
	}

	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	client, err := client.NewXataClient(apiKey, "")
	if err != nil {
		return err
	}

	resp, err := client.CreateWorkspace(c.Context, spec.CreateWorkspaceJSONRequestBody{
		Name: workspaceName,
		Slug: slug.Make(workspaceName),
	})
	return printResponse(c, resp, err)
}

func deleteWorkspace(c *cli.Context) error {
	workspaceID := c.Args().Get(0)
	if len(workspaceID) == 0 {
		return fmt.Errorf("please specify a workspace ID")
	}
	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	client, err := client.NewXataClient(apiKey, "")
	if err != nil {
		return err
	}

	resp, err := client.DeleteWorkspace(c.Context, spec.WorkspaceIDParam(workspaceID))
	return printResponse(c, resp, err)
}
