package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/xata/cli/client"
	"github.com/xata/cli/client/spec"
	"github.com/xata/cli/config"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

func HistoryCommand(c *cli.Context) error {
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

	fmt.Printf("Migrations log of [%s]:\n\n", dbName)
	err = printHistory(c.Context, client, dbName, branch, "", c.Bool("follow"))
	if err != nil {
		return err
	}

	return nil
}

func PrintMigration(migration spec.BranchMigration) {
	blue := color.New(color.BgBlue).Add(color.FgWhite)
	red := color.New(color.BgRed).Add(color.FgWhite)
	yellow := color.New(color.BgYellow).Add(color.FgBlack)
	yellowFG := color.New(color.FgYellow)
	indent := "  "

	if migration.Title != nil {
		yellowFG.Printf("* %s [status: %s]\n", *migration.Title, migration.Status)
	}
	if migration.Id != nil {
		fmt.Printf("ID: %s\n", *migration.Id)
	}
	if migration.LastGitRevision != nil {
		fmt.Printf("git commit sha1: %s", *migration.LastGitRevision)
		if migration.LocalChanges {
			fmt.Printf(" + local changes")
		}
		fmt.Println()
	}

	if migration.CreatedAt != nil {
		fmt.Printf("Date: %s\n", time.Time(*migration.CreatedAt))
	}

	if migration.NewTables != nil {
		for tableName := range migration.NewTables.AdditionalProperties {
			blue.Printf(" CREATE table ")
			fmt.Printf(" %s\n", tableName)
		}
	}
	if migration.RemovedTables != nil {
		for _, tableName := range *migration.RemovedTables {
			red.Printf(" DELETE table ")
			fmt.Printf(" %s\n", tableName)
		}
	}
	if migration.RenamedTables != nil {
		for _, tableRename := range *migration.RenamedTables {
			blue.Printf(" RENAME table ")
			fmt.Printf(" %s TO %s\n", tableRename.OldName, tableRename.NewName)
		}
	}

	if migration.TableMigrations != nil {
		for tableName, tableMigration := range migration.TableMigrations.AdditionalProperties {
			fmt.Printf("Table [%s]:\n", tableName)
			if tableMigration.NewColumns != nil {
				for columnName := range tableMigration.NewColumns.AdditionalProperties {
					fmt.Print(indent)
					blue.Printf(" ADD column ")
					fmt.Printf(" %s\n", columnName)
				}
			}
			if tableMigration.RemovedColumns != nil {
				for _, columnName := range *tableMigration.RemovedColumns {
					fmt.Print(indent)
					red.Printf(" DELETE column ")
					fmt.Printf(" %s\n", columnName)
				}
			}
			if tableMigration.ModifiedColumns != nil {
				for _, column := range *tableMigration.ModifiedColumns {
					fmt.Print(indent)
					yellow.Printf(" MODIFY column ")
					fmt.Printf(" %s\n", column.Old.Name)
				}
			}
		}
	}
}

func checkHistoryResponse(history *spec.GetBranchMigrationHistoryResponse) error {
	if history.JSON401 != nil {
		return ErrorUnauthorized{message: history.JSON401.Message}
	}
	if history.JSON400 != nil {
		return fmt.Errorf("Error getting history: %s", history.JSON400.Message)
	}
	if history.JSON404 != nil {
		return fmt.Errorf("Error getting history: %s", history.JSON404.Message)
	}

	if history.StatusCode() != http.StatusOK {
		return fmt.Errorf("Error getting history: %s", history.Status())
	}
	if history.JSON200 == nil {
		return fmt.Errorf("Error getting history: 200 OK unexpected response body")
	}
	return nil
}

func printHistory(ctx context.Context, client *spec.ClientWithResponses, dbName, branchName, startFromID string, follow bool) error {
	startFrom := startFromID
	var originBase *spec.StartedFromMetadata
	// TODO: this needs to be updated to use StartedFromBranch
	for {
		req := spec.GetBranchMigrationHistoryJSONRequestBody{
			StartFrom: &startFrom,
		}
		dbbranch := spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, branchName))
		history, err := client.GetBranchMigrationHistoryWithResponse(ctx, dbbranch, req)
		if err != nil {
			return fmt.Errorf("Error getting migrations: %w", err)
		}
		err = checkHistoryResponse(history)
		if err != nil {
			return err
		}

		migrations := *history.JSON200.Migrations
		originBase = history.JSON200.StartedFrom
		for _, migration := range migrations {
			PrintMigration(migration)
			fmt.Println()
		}
		if len(migrations) == 0 {
			break
		}
		if migrations[len(migrations)-1].ParentID == nil {
			break
		}
		startFrom = *migrations[len(migrations)-1].ParentID
	}

	if originBase != nil {
		yellowFG := color.New(color.FgYellow)
		yellowFG.Printf("➜ created by copying schema from branch [%s @ %s]\n", originBase.BranchName, originBase.MigrationID)
	}

	if follow && originBase != nil {
		fmt.Println()
		printHistory(ctx, client, dbName, string(originBase.BranchName), originBase.MigrationID, follow)
	}
	return nil
}
