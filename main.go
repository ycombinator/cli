package main

import (
	"fmt"
	"hash/maphash"
	"math/rand"
	"os"
	"path"

	"github.com/xataio/cli/buildvar"
	"github.com/xataio/cli/cmd"
	"github.com/xataio/cli/config"
	"github.com/xataio/cli/filesystem"

	"github.com/urfave/cli/v2"
)

func printVersion(c *cli.Context) error {
	fmt.Printf("%s %s\n", c.App.Name, buildvar.Version)
	return nil
}

func getSchemaStatus(c *cli.Context) error {
	dir := c.String("dir")

	settings, err := cmd.ReadSettings(dir)
	if err != nil {
		return err
	}

	schemaFile := path.Join(dir, "schema.json")
	if settings.SchemaFileFormat == cmd.SettingsYAML {
		schemaFile = path.Join(dir, "schema.yaml")
	}

	exists, errr := filesystem.FileExists(schemaFile)
	if errr != nil {
		return fmt.Errorf("Error checking if schema file exists: %w.", err)
	}
	if !exists {
		return fmt.Errorf("Schema file [%s] doesn't exist\n", schemaFile)
	}

	sha, err := cmd.GitGetLastSHA()
	if err != nil {
		return fmt.Errorf("Error getting git sha: %s", err)
	}

	localChanges, err := cmd.GitHasLocalChanges(schemaFile)
	if err != nil {
		return fmt.Errorf("Error checking for local changes: %s", err)
	}
	status := ""
	if localChanges {
		status = "modified"
	}
	fmt.Printf("%s: %s %s\n", schemaFile, sha, status)
	return nil
}

func main() {
	// initialize global seed via runtime.fastrand
	rand.Seed(int64(new(maphash.Hash).Sum64()))

	app := &cli.App{
		Name:  "Xata CLI",
		Usage: "Command Line Interface for the xata.io serverless database service.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "dir",
				Aliases: []string{"d"},
				Value:   "xata",
				Usage:   "The `DIR` containing the Xata files, including the schema",
			},
			&cli.BoolFlag{
				Name:  "nocolor",
				Usage: "Disable colors",
			},
			&cli.BoolFlag{
				Name:  "lightbg",
				Usage: "Switch color scheme to light colors",
			},
			&cli.StringFlag{
				Name:  config.ArgKey,
				Usage: "Xata global config directory",
			},
		},

		Commands: []*cli.Command{
			{
				Name:        "auth",
				Usage:       "Configure CLI authentication",
				Subcommands: cmd.GetAuthSubcommands(),
			},
			{
				Name:   "init",
				Usage:  "Start a new database in the current folder.",
				Action: cmd.InitCommand,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Overwrite files",
					},
					&cli.BoolFlag{
						Name:  "yaml",
						Usage: "Use YAML format for the schema file.",
					},
					&cli.StringFlag{
						Name:  "dbname",
						Usage: "The name of the database to create.",
					},
					&cli.StringFlag{
						Name:  "workspaceid",
						Usage: "The id of the workspace to use.",
					},
				},
			},
			{
				Name:   "deploy",
				Usage:  "Deploy database to xata.io",
				Action: cmd.DeployCommand,
			},
			{
				Name:   "pull",
				Usage:  "Pull schema file from the remote branch",
				Action: cmd.PullCommand,
			},
			{
				Name:   "log",
				Usage:  "Get the log history of your database.",
				Action: cmd.HistoryCommand,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "follow",
						Usage: "Follow log history across branches.",
					},
				},
			},
			{
				Name:   "shell",
				Usage:  "Interactive shell to explore your data.",
				Action: cmd.ShellCommand,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "dbname",
						Usage: "The database to select.",
					},
					&cli.BoolFlag{
						Name:    "current",
						Aliases: []string{"c"},
						Usage:   "Use current database and branch as a prefix",
					},
					&cli.BoolFlag{
						Name:  "nopretty",
						Usage: "Disable automatic pretty-printing.",
					},
				},
			},
			{
				Name:  "schema",
				Usage: "Operations on the schema file",
				Subcommands: []*cli.Command{
					{
						Name:   "status",
						Usage:  "Get schema status",
						Action: getSchemaStatus,
					},
				},
			},
			{
				Name:        "dbs",
				Usage:       "Operations on databases",
				Subcommands: cmd.GetDBsSubcommands(),
			},
			{
				Name:        "branches",
				Usage:       "Operations on branches",
				Subcommands: cmd.GetBranchesSubcommands(),
			},
			{
				Name:        "workspaces",
				Usage:       "Operations on workspaces",
				Subcommands: cmd.GetWorkspacesSubcommands(),
			},
			{
				Name:   "random-data",
				Usage:  "Insert random data in table.",
				Action: cmd.GenerateRandomData,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "records",
						Aliases: []string{"n"},
						Usage:   "Number of records to generate per table.",
						Value:   25,
					},
					&cli.StringSliceFlag{
						Name:    "table",
						Aliases: []string{"t"},
						Usage:   "Table in which to add data (default: all). Can be specified multiple times.",
					},
				},
			},
			{
				Name:   "version",
				Usage:  "Build information",
				Action: printVersion,
			},
		},
	}
	app.EnableBashCompletion = true
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
