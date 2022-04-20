package cmd

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"

	lorem "github.com/drhodes/golorem"
	petname "github.com/dustinkirkland/golang-petname"
	"github.com/urfave/cli/v2"
)

func isTableSelected(tables []string, tableName string) bool {
	if len(tables) == 0 {
		return true
	}
	for _, name := range tables {
		if tableName == name {
			return true
		}
	}
	return false
}

func generateRandomDoc(columns []spec.Column) *map[string]interface{} {
	doc := map[string]interface{}{}
	for _, column := range columns {
		switch column.Type {
		case spec.ColumnTypeString:
			doc[column.Name] = petname.Generate(2, " ")
		case spec.ColumnTypeBool:
			doc[column.Name] = (rand.Intn(2) == 1)
		case spec.ColumnTypeInt:
			doc[column.Name] = rand.Intn(100)
		case spec.ColumnTypeFloat:
			doc[column.Name] = math.Floor(rand.Float64()*100) / 100
		case spec.ColumnTypeObject:
			doc[column.Name] = generateRandomDoc(column.Columns)
		case spec.ColumnTypeEmail:
			doc[column.Name] = fmt.Sprintf("%s@%s.pets", petname.Adjective(), petname.Generate(1, ""))
		case spec.ColumnTypeText:
			doc[column.Name] = lorem.Paragraph(2, 4)
		case spec.ColumnTypeMultiple:
			values := []string{}
			nValues := rand.Intn(3) + 1
			for i := 0; i < nValues; i++ {
				values = append(values, petname.Generate(2, " "))
			}
			doc[column.Name] = values
		}
	}
	return &doc
}

func GenerateRandomData(c *cli.Context) error {
	tables := c.StringSlice("table")
	numberOfRecords := c.Int("records")

	rand.Seed(time.Now().UnixNano())

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

	baseBranch, err := client.GetBranchDetailsWithResponse(c.Context, dbbranch)
	if err != nil {
		return err
	}

	if baseBranch.StatusCode() > 299 {
		return fmt.Errorf("getting branch details: %s", baseBranch.Status())
	}

	for _, table := range baseBranch.JSON200.Schema.Tables {
		if !isTableSelected(tables, string(table.Name)) {
			continue
		}

		documents := []map[string]interface{}{}
		for i := 0; i < numberOfRecords; i++ {
			doc := generateRandomDoc(table.Columns)
			documents = append(documents, *doc)
		}

		req := spec.BulkInsertTableRecordsJSONRequestBody{
			Records: documents,
		}
		_, err = client.BulkInsertTableRecords(c.Context, dbbranch, spec.TableNameParam(table.Name), req)
		if err != nil {
			return err
		}

		fmt.Printf("Inserted %d random records in table %s\n", numberOfRecords, table.Name)
	}

	return nil
}
