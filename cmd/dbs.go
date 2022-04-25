package cmd

import (
	"fmt"

	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"

	"github.com/urfave/cli/v2"
)

func GetDBsSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "list",
			Usage:  "List databases",
			Action: listDBs,
		},
		{
			Name:   "create",
			Usage:  "Create database",
			Action: createDB,
		},
		{
			Name:   "delete",
			Usage:  "Delete database",
			Action: deleteDB,
		},
	}
}

func listDBs(c *cli.Context) error {
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

	cr := spec.ClientWithResponses{ClientInterface: client}
	resp, err := cr.GetDatabaseListWithResponse(c.Context)
	return printResponse(c, resp, resp.Body, err, func() error {
		data := resp.JSON200
		if data == nil {
			return fmt.Errorf("Unexpected server response %s", resp.Status())
		}
		arr := *data.Databases
		table := make([][]interface{}, len(arr))
		for i := 0; i < len(arr); i++ {
			database := arr[i]
			table[i] = []interface{}{database.Name, database.CreatedAt, database.NumberOfBranches}
		}
		printTable([]string{"Database name", "Created at", "Number of branches"}, table)
		return nil
	})
}

func createDB(c *cli.Context) error {
	dbName := c.Args().Get(0)
	if len(dbName) == 0 {
		return fmt.Errorf("please specify a DB name")
	}
	if !spec.IsValidIdentifier(dbName) {
		return fmt.Errorf("Invalid DB name")
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

	cr := spec.ClientWithResponses{ClientInterface: client}
	resp, err := cr.CreateDatabaseWithResponse(c.Context, spec.DBNameParam(dbName),
		spec.CreateDatabaseJSONRequestBody{})
	return printResponse(c, resp, resp.Body, err, func() error {
		fmt.Println("Database successfully created")
		return nil
	})
}

func deleteDB(c *cli.Context) error {
	dbName := c.Args().Get(0)
	if len(dbName) == 0 {
		return fmt.Errorf("please specify a DB name")
	}
	if !spec.IsValidIdentifier(dbName) {
		return fmt.Errorf("Invalid DB name")
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

	cr := spec.ClientWithResponses{ClientInterface: client}
	resp, err := cr.DeleteDatabaseWithResponse(c.Context, spec.DBNameParam(dbName))
	return printResponse(c, resp, resp.Body, err, func() error {
		fmt.Println("Database successfully deleted")
		return nil
	})
}
