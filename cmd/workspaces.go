package cmd

import (
	"fmt"

	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"

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

	cr := spec.ClientWithResponses{ClientInterface: client}
	resp, err := cr.GetWorkspacesListWithResponse(c.Context)
	return printResponse(c, resp, resp.Body, err, func() error {
		data := resp.JSON200
		if data == nil {
			return fmt.Errorf("Unexpected server response %s", resp.Status())
		}
		table := make([][]interface{}, len(data.Workspaces))
		for i := 0; i < len(data.Workspaces); i++ {
			workspace := data.Workspaces[i]
			table[i] = []interface{}{workspace.Name, workspace.Id, workspace.Role}
		}
		printTable([]string{"Workspace name", "Id", "Role"}, table)
		return nil
	})
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

	cr := spec.ClientWithResponses{ClientInterface: client}
	resp, err := cr.CreateWorkspaceWithResponse(c.Context, spec.CreateWorkspaceJSONRequestBody{
		Name: workspaceName,
		Slug: slug.Make(workspaceName),
	})
	return printResponse(c, resp, resp.Body, err, func() error {
		fmt.Println("Workspace successfully created")
		return nil
	})
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

	cr := spec.ClientWithResponses{ClientInterface: client}
	resp, err := cr.DeleteWorkspaceWithResponse(c.Context, spec.WorkspaceIDParam(workspaceID))
	return printResponse(c, resp, resp.Body, err, func() error {
		fmt.Println("Workspace successfully deleted")
		return nil
	})
}
