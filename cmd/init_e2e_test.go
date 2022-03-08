package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/Netflix/go-expect"
	"github.com/stretchr/testify/require"
	"github.com/xata/cli/client/spec"
	"gopkg.in/yaml.v3"
)

func chdir(t *testing.T, dir string) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(dir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = os.Chdir(wd)
		require.NoError(t, err)
	})
}

func TestInit(t *testing.T) {
	config, configDir, workspaceID := createTestWorkspace(t)

	dir := t.TempDir()
	chdir(t, dir)

	tests := []struct {
		name      string
		args      []string
		procedure func(t *testing.T, c *expect.Console)
		exitCode  int
	}{
		{
			name: "simple init",
			args: []string{"init", "--workspaceid", workspaceID},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("New database name:")
				require.NoError(t, err)

				_, err = c.SendLine("test")
				require.NoError(t, err)

				_, err = c.ExpectString("Init done. Edit xata/schema.json to get started.")
				require.NoError(t, err)
			},
			exitCode: 0,
		},
		{
			name: "run xata deploy",
			args: []string{"deploy"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Database [test/main] created")
				require.NoError(t, err)

				_, err = c.ExpectString("Apply the above migration?")
				require.NoError(t, err)

				_, err = c.SendLine("Y")
				require.NoError(t, err)

				_, err = c.ExpectString("Done.")
				require.NoError(t, err)
			},
		},
		{
			name: "second init asks for -f",
			args: []string{"init", "--workspaceid", workspaceID},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Directory `xata` already exists, so I am not overwriting it. Use -f if you are sure.")
				require.NoError(t, err)
			},
			exitCode: 1,
		},
		{
			name: "second init with -f, pull DB",
			args: []string{"init", "--workspaceid", workspaceID, "-f"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Do you want to pull from an existing database?")
				require.NoError(t, err)

				_, err = c.SendLine("test")
				require.NoError(t, err)

				_, err = c.ExpectString("Pulling database schema...")
				require.NoError(t, err)

				_, err = c.ExpectString("Init done. Edit xata/schema.json to get started.")
				require.NoError(t, err)
			},
			exitCode: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, cmd := startCommand(t, configDir, config.TestBinaryPath, test.args...)
			defer c.Close()

			test.procedure(t, c)

			err := cmd.Wait()
			require.Equal(t, test.exitCode, exitCodeFromError(t, err))
		})
	}

}

func runInitCommand(t *testing.T, config TestConfig, configDir string, workspaceID, dbName string) {
	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "init", "--workspaceid", workspaceID)
	defer c.Close()

	_, err := c.ExpectString("New database name:")
	require.NoError(t, err)

	_, err = c.SendLine(dbName)
	require.NoError(t, err)

	_, err = c.ExpectString("Init done. Edit xata/schema.json to get started.")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)
}

func runDeployCommandAfterInit(t *testing.T, config TestConfig, configDir string, workspaceID, dbName string) {
	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "deploy")
	defer c.Close()

	_, err := c.ExpectString(fmt.Sprintf("Database [%s/main] created", dbName))
	require.NoError(t, err)

	_, err = c.ExpectString("Apply the above migration?")
	require.NoError(t, err)

	_, err = c.SendLine("Y")
	require.NoError(t, err)

	_, err = c.ExpectString("Done.")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)
}

func TestInitYAML(t *testing.T) {
	config, configDir, workspaceID := createTestWorkspace(t)

	dir := t.TempDir()
	chdir(t, dir)

	tests := []struct {
		name      string
		args      []string
		procedure func(t *testing.T, c *expect.Console)
		exitCode  int
	}{
		{
			name: "simple init yaml",
			args: []string{"init", "--workspaceid", workspaceID, "--yaml"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("New database name:")
				require.NoError(t, err)

				_, err = c.SendLine("test")
				require.NoError(t, err)

				_, err = c.ExpectString("Init done. Edit xata/schema.yaml to get started.")
				require.NoError(t, err)
			},
			exitCode: 0,
		},
		{
			name: "run xata deploy",
			args: []string{"deploy"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Database [test/main] created")
				require.NoError(t, err)

				_, err = c.ExpectString("Apply the above migration?")
				require.NoError(t, err)

				_, err = c.SendLine("Y")
				require.NoError(t, err)

				_, err = c.ExpectString("Done.")
				require.NoError(t, err)
			},
		},
		{
			name: "run xata deploy again, should be up to date",
			args: []string{"deploy"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Your schema is up to date.")
				require.NoError(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, cmd := startCommand(t, configDir, config.TestBinaryPath, test.args...)
			defer c.Close()

			test.procedure(t, c)

			err := cmd.Wait()
			require.Equal(t, test.exitCode, exitCodeFromError(t, err))
		})
	}

	// Add a column to the schema
	schemaFile := path.Join(dir, "xata", "schema.yaml")
	var schema spec.Schema
	file, err := os.Open(schemaFile)
	require.NoError(t, err)

	bytes, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	err = yaml.Unmarshal(bytes, &schema)
	require.NoError(t, err)

	schema.Tables[0].Columns = append(schema.Tables[0].Columns, spec.Column{
		Name: "newColumn",
		Type: spec.ColumnTypeString,
	})

	bytes, err = yaml.Marshal(schema)
	require.NoError(t, err)

	err = ioutil.WriteFile(schemaFile, bytes, 0644)
	require.NoError(t, err)

	tests = []struct {
		name      string
		args      []string
		procedure func(t *testing.T, c *expect.Console)
		exitCode  int
	}{
		{
			name: "run xata deploy",
			args: []string{"--nocolor", "deploy"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Migration plan preview:")
				require.NoError(t, err)

				_, err = c.ExpectString("Table [teams]:")
				require.NoError(t, err)

				_, err = c.ExpectString("ADD column  newColumn")
				require.NoError(t, err)

				_, err = c.ExpectString("Apply the above migration?")
				require.NoError(t, err)

				_, err = c.SendLine("Y")
				require.NoError(t, err)

				_, err = c.ExpectString("Done.")
				require.NoError(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, cmd := startCommand(t, configDir, config.TestBinaryPath, test.args...)
			defer c.Close()

			test.procedure(t, c)

			err := cmd.Wait()
			require.Equal(t, test.exitCode, exitCodeFromError(t, err))
		})
	}
}

func TestInitDifferentBranch(t *testing.T) {
	config, configDir, workspaceID := createTestWorkspace(t)

	dir := t.TempDir()
	chdir(t, dir)

	setupGitAndDefaultDB(t, config, configDir, workspaceID)

	cmd := exec.Command("git", "checkout", "-b", "new_feature")
	err := cmd.Run()
	require.NoError(t, err)

	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "init", "--workspaceid", workspaceID, "-f")
	defer c.Close()

	_, err = c.ExpectString("Do you want to pull from an existing database?")
	require.NoError(t, err)

	_, err = c.SendLine("test")
	require.NoError(t, err)

	_, err = c.ExpectString("Pulling database schema...")
	require.NoError(t, err)

	_, err = c.ExpectString("Branch [test:new_feature] does not exist. Would you like to pull the schema from another branch?")
	require.NoError(t, err)

	_, err = c.SendLine("Y")
	require.NoError(t, err)

	_, err = c.ExpectString("From which branch should I pull the schema?")
	require.NoError(t, err)

	_, err = c.SendLine("main")
	require.NoError(t, err)

	_, err = c.ExpectString("Pulling schema from branch [test:main]")
	require.NoError(t, err)

	_, err = c.ExpectString("Pulled schema written to xata/schema.json")
	require.NoError(t, err)

	_, err = c.ExpectString("Init done. Edit xata/schema.json to get started.")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)
}
