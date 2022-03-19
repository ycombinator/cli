package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"
)

func loadBranch(c *cli.Context) error {
	branchURL := c.String("branchUrl")
	dir := c.String("input")
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
	cr := &spec.ClientWithResponses{ClientInterface: client}

	assumeMigrationYes := false

	// Make sure the destination branch exists
	dbbranch := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbname, branch))
	resp, err := cr.GetBranchDetailsWithResponse(c.Context, dbbranch)
	if err != nil {
		return err
	}
	if resp.JSON404 != nil {
		fmt.Printf("Branch [%s] does not exist. Creating it...\n", dbbranch)
		assumeMigrationYes = true

		err = ensureBranchExist(c.Context, client, dbname, branch)
		if err != nil {
			return fmt.Errorf("Error creating branch [%s]: %w", dbbranch, err)
		}
	} else {
		err = checkBranchDetails(resp)
		if err != nil {
			return err
		}
	}

	// Migrate to the schema saved in the file
	schemaFile := path.Join(dir, "schema.json")
	schema, err := readSchemaFromFile(schemaFile)
	if err != nil {
		return err
	}

	err = migrateBranchToSchema(c, client, dbbranch, schema, assumeMigrationYes)
	if err != nil {
		return err
	}

	// Load the data
	for _, table := range schema.Tables {
		err = loadTableFile(c.Context, client, dir, dbbranch, table)
		if err != nil {
			return fmt.Errorf("Error loading table [%s]: %w", table.Name, err)
		}
	}
	return nil
}

func loadTableFile(ctx context.Context, client *spec.Client, dir string,
	dbbranch spec.DBBranchNameParam, table spec.Table) error {

	tableFileName := path.Join(dir, table.Name+".ndjson")
	tableFile, err := os.Open(tableFileName)
	if err != nil {
		return fmt.Errorf("opening table file [%s]: %w", tableFileName, err)
	}
	defer tableFile.Close()

	scanner := bufio.NewScanner(tableFile)
	lineNo := 0
	for scanner.Scan() {
		lineNo += 1
		recBytes := scanner.Bytes()
		var record map[string]interface{}
		err := json.Unmarshal(recBytes, &record)
		if err != nil {
			return fmt.Errorf("parsing json at line %d: %w", lineNo, err)
		}

		idVal, ok := record["id"]
		if !ok {
			return fmt.Errorf("id record not found at line %d", lineNo)
		}
		recID, ok := idVal.(string)
		if !ok {
			return fmt.Errorf("id record should be string at line %d", lineNo)
		}
		delete(record, "id")
		delete(record, "xata")

		record, err = removeLinkValues(table.Columns, record)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}

		// TODO: use bulk insert here once we support bulks with ID specified
		resp, err := client.InsertRecordWithID(ctx, dbbranch,
			spec.TableNameParam(table.Name),
			spec.RecordIDParam(recID),
			&spec.InsertRecordWithIDParams{}, record)
		if err != nil {
			return fmt.Errorf("inserting line %d: %w", lineNo, err)
		}
		if resp.StatusCode > 299 {
			return fmt.Errorf("inserting line %d: %d %s", lineNo, resp.StatusCode, responseToError(resp))
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning file: %w", err)
	}
	return nil
}

func removeLinkValues(columns []spec.Column, record map[string]interface{}) (map[string]interface{}, error) {
	for _, col := range columns {
		if col.Type == spec.ColumnTypeLink {
			delete(record, col.Name)
		}
		if col.Type == spec.ColumnTypeObject {
			subRecord, exists := record[col.Name]
			if !exists {
				continue
			}
			subRecordMap, ok := subRecord.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("unexpected non-object for key %s", col.Name)
			}
			var err error
			record[col.Name], err = removeLinkValues(col.Columns, subRecordMap)
			if err != nil {
				return nil, err
			}
		}
	}
	return record, nil
}

func readSchemaFromFile(schemaFile string) (spec.Schema, error) {
	jsonFile, err := os.Open(schemaFile)
	if err != nil {
		return spec.Schema{}, fmt.Errorf("reading schema file [%s]: %w", schemaFile, err)
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return spec.Schema{}, fmt.Errorf("reading schema file [%s]: %w", schemaFile, err)
	}

	var schema spec.Schema
	err = json.Unmarshal([]byte(byteValue), &schema)
	if err != nil {
		return spec.Schema{}, fmt.Errorf("parsing schema json file [%s]: %w", schemaFile, err)
	}

	return schema, nil
}

func migrateBranchToSchema(c *cli.Context, client *spec.Client,
	dbbranch spec.DBBranchNameParam, schema spec.Schema, assumeYes bool) error {

	cr := &spec.ClientWithResponses{ClientInterface: client}

	respPlan, err := cr.GetBranchMigrationPlanWithResponse(c.Context,
		dbbranch,
		spec.GetBranchMigrationPlanJSONRequestBody(schema))
	if err != nil {
		return fmt.Errorf("Error getting migration plan: %w", err)
	}
	if err = checkBranchMigrationPlan(respPlan); err != nil {
		return err
	}

	plan := respPlan.JSON200
	if (plan.Migration.NewTables == nil || len(plan.Migration.NewTables.AdditionalProperties) == 0) &&
		plan.Migration.RemovedTables == nil &&
		plan.Migration.TableMigrations == nil {
		fmt.Println("Schema is already up to date")
		return nil
	}

	fmt.Printf("Apply a schema migration first. Migration plan preview:\n\n")
	PrintMigration(plan.Migration, c.Bool("nocolor"))
	fmt.Println()

	if !assumeYes {
		var yes bool
		prompt := &survey.Confirm{
			Message: "Apply the above migration?",
			Default: true,
		}
		survey.AskOne(prompt, &yes)
		if !yes {
			return fmt.Errorf("Execution cancelled")
		}
	}

	now := spec.DateTime(time.Now())
	plan.Migration.CreatedAt = &now
	mResp, err := cr.ExecuteBranchMigrationPlanWithResponse(c.Context,
		dbbranch,
		spec.ExecuteBranchMigrationPlanJSONRequestBody(*plan))
	if err != nil {
		return fmt.Errorf("Error executing migration: %w", err)
	}
	if mResp.StatusCode() > 299 {
		return fmt.Errorf("Error executing migration: %s", mResp.Status())
	}

	fmt.Println("Schema migrated.")

	return nil
}

// ensureBranchExist creates a branch if it does not exist. If the parent database doesn't
// exist, it is created as well.
func ensureBranchExist(ctx context.Context, client *spec.Client, dbname, branch string) error {
	dbbranch := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbname, branch))
	clientWithResponses := &spec.ClientWithResponses{ClientInterface: client}

	resp, err := clientWithResponses.GetBranchListWithResponse(ctx, spec.DBNameParam(dbname))
	if err != nil {
		return err
	}
	if resp.JSON404 != nil {
		resp, err := client.CreateDatabase(ctx, spec.DBNameParam(dbname), spec.CreateDatabaseJSONRequestBody{
			BranchName: &branch,
		})
		if err != nil {
			return err
		}
		if resp.StatusCode > 299 {
			return responseToError(resp)
		}
		fmt.Printf("Created database [%s] with branch [%s]\n", dbname, branch)
		return nil
	}
	if err = checkBranchesList(resp); err != nil {
		return err
	}

	if len(resp.JSON200.Branches) == 0 {
		return fmt.Errorf("Unexpected: no branches found in database [%s]", dbname)
	}

	response, err := client.CreateBranch(ctx, dbbranch, &spec.CreateBranchParams{}, spec.CreateBranchJSONRequestBody{
		From: &resp.JSON200.Branches[0].Name,
	})
	if err != nil {
		return err
	}
	if response.StatusCode > 299 {
		return responseToError(response)
	}
	fmt.Printf("Created branch [%s]\n", dbbranch)
	return nil
}

func checkBranchesList(resp *spec.GetBranchListResponse) error {
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
		return fmt.Errorf("listing branches: %s", resp.Status())
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("listing branches: 200 OK unexpected response body")
	}
	return nil
}

func LoadBranchSubcommand() *cli.Command {
	return &cli.Command{
		Name:   "load",
		Usage:  "Load the contents of a dump folder to a branch.",
		Action: loadBranch,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Usage:    "The input directory containing the branch dump.",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "branchUrl",
				Usage:    "The URL of the branch in which to load the data, in the form: https://{workspaceid}.xata.sh/db/{database}:{branch}",
				Required: true,
			},
		},
	}
}
