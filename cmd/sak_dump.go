package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/tidwall/pretty"
	"github.com/urfave/cli/v2"
	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"
)

const dumpPageSize int = 10

func dumpBranch(c *cli.Context) error {

	branchURL := c.String("branchUrl")
	workspace, dbname, branch, err := parseBranchURL(branchURL)
	if err != nil {
		return fmt.Errorf("Invalid branch URL: %w", err)
	}

	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	client, err := client.NewXataClient(apiKey, workspace)
	if err != nil {
		return err
	}
	clientWithResponses := &spec.ClientWithResponses{ClientInterface: client}

	dir := c.String("output")
	if _, err := os.Stat(dir); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("Output directory %s already exists", dir)
	}

	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("Error creating output directory %s: %w", dir, err)
	}

	dbbranch := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbname, branch))
	resp, err := clientWithResponses.GetBranchDetailsWithResponse(c.Context, dbbranch)
	if err != nil {
		return err
	}

	baseBranch := resp.JSON200
	err = dumpSchemaFile(dir, baseBranch.Schema)
	if err != nil {
		return err
	}

	for _, table := range baseBranch.Schema.Tables {
		err = dumpTableFile(clientWithResponses, dir,
			fmt.Sprintf("%s:%s", dbname, branch), table)
		if err != nil {
			return fmt.Errorf("Error dumping table %s: %w", table.Name, err)
		}
	}

	return nil
}

func dumpSchemaFile(dir string, schema spec.Schema) error {
	file, err := json.MarshalIndent(schema, "", " ")
	if err != nil {
		return err
	}
	file = pretty.Pretty(file)

	err = ioutil.WriteFile(path.Join(dir, "schema.json"), file, 0644)
	if err != nil {
		return fmt.Errorf("writing schema file: %w", err)
	}

	fmt.Printf("Schema file written in: %s\n", path.Join(dir, "schema.json"))
	return nil
}

func dumpTableFile(client *spec.ClientWithResponses, dir string, dbBranch string, table spec.Table) error {

	fileName := fmt.Sprintf("%s.ndjson", table.Name)
	outputFile, err := os.OpenFile(path.Join(dir, fileName), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file for writing: %w", err)
	}

	more := true
	cursor := ""
	pageSize := dumpPageSize
	for more {
		page := spec.PageConfig{
			Size: &pageSize,
		}
		if cursor != "" {
			page.After = &cursor
		}
		resp, err := client.QueryTableWithResponse(context.TODO(),
			spec.DBBranchNameParam(dbBranch),
			spec.TableNameParam(table.Name),
			spec.QueryTableJSONRequestBody{
				Page: &page,
			})
		if err != nil {
			return fmt.Errorf("error quering: %w", err)
		}
		err = checkQueryDetails(resp)
		if err != nil {
			return err
		}

		for _, record := range resp.JSON200.Records {
			recordMap := record.AdditionalProperties
			if recordMap == nil {
				recordMap = map[string]interface{}{}
			}
			recordMap["id"] = record.Id
			recordMap["xata"] = record.Xata
			recBytes, err := json.Marshal(recordMap)
			if err != nil {
				return err
			}
			_, err = outputFile.Write(recBytes)
			if err != nil {
				return err
			}
			_, err = outputFile.Write([]byte("\n"))
			if err != nil {
				return err
			}
		}

		more = resp.JSON200.Meta.Page.More
		cursor = resp.JSON200.Meta.Page.Cursor
		if more {
			fmt.Println("Continuing with", cursor)
		} else {
			fmt.Printf("Table %s dumped to %s\n", table.Name, fileName)
		}
	}
	return nil
}

func checkQueryDetails(resp *spec.QueryTableResponse) error {
	if resp.JSON401 != nil {
		return ErrorUnauthorized{message: resp.JSON401.Message}
	}
	if resp.JSON400 != nil {
		return fmt.Errorf(resp.JSON400.Message)
	}
	if resp.JSON404 != nil {
		return fmt.Errorf(resp.JSON404.Message)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("querying: %s", resp.Status())
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("querying: 200 OK unexpected response body")
	}
	return nil
}

func DumpBranchSubcommand() *cli.Command {
	return &cli.Command{
		Name:   "dump",
		Usage:  "Dump the contents of a given database branch to a folder.",
		Action: dumpBranch,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "output",
				Usage:    "The output directory to write the dump to.",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "branchUrl",
				Usage:    "The URL of the branch to dump, in the form: https://{workspaceid}.xata.sh/db/{database}:{branch}",
				Required: true,
			},
		},
	}
}
