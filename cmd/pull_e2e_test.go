package cmd

import (
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/Netflix/go-expect"
	"github.com/stretchr/testify/require"
)

func setupGitAndDefaultDB(t *testing.T, config TestConfig, configDir, workspaceID string) {
	cmd := exec.Command("git", "init")
	err := cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "branch", "-m", "main")
	err = cmd.Run()
	require.NoError(t, err)

	runInitCommand(t, config, configDir, workspaceID, "test")
	runDeployCommandAfterInit(t, config, configDir, workspaceID, "test")

	cmd = exec.Command("git", "config", "user.name", "Test")
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "config", "user.email", "noreply@xata.io")
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "add", "-A")
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	err = cmd.Run()
	require.NoError(t, err)
}

func TestPull(t *testing.T) {
	config, configDir, workspaceID := createTestWorkspace(t)

	dir := t.TempDir()
	chdir(t, dir)

	setupGitAndDefaultDB(t, config, configDir, workspaceID)

	// Delete schema file
	schemaFile := path.Join(dir, "xata", "schema.json")
	err := os.Remove(schemaFile)
	require.NoError(t, err)

	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "pull")
	defer c.Close()

	_, err = c.ExpectString("Pulling schema from branch [test:main]")
	require.NoError(t, err)

	_, err = c.ExpectString("Pulled schema written to xata/schema.json")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)

	// Schema should exist again
	_, err = os.Stat(schemaFile)
	require.NoError(t, err)

	// Run again, this time a backup is created
	c, cmd = startCommand(t, configDir, config.TestBinaryPath, "pull")
	defer c.Close()

	_, err = c.ExpectString("Pulling schema from branch [test:main]")
	require.NoError(t, err)

	_, err = c.ExpectString("Your existing schema file has been backed up to xata/schema.json.bak")
	require.NoError(t, err)

	_, err = c.ExpectString("Pulled schema written to xata/schema.json")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)
}

func TestPullFromDifferentBranch(t *testing.T) {
	config, configDir, workspaceID := createTestWorkspace(t)

	dir := t.TempDir()
	chdir(t, dir)

	setupGitAndDefaultDB(t, config, configDir, workspaceID)

	cmd := exec.Command("git", "checkout", "-b", "new_feature")
	err := cmd.Run()
	require.NoError(t, err)

	tests := []struct {
		name      string
		args      []string
		procedure func(t *testing.T, c *expect.Console)
		exitCode  int
	}{
		{
			name: "run xata pull, but answer No",
			args: []string{"pull"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Branch [test:new_feature] does not exist. Would you like to pull the schema from another branch?")
				require.NoError(t, err)

				_, err = c.SendLine("n")
				require.NoError(t, err)

				_, err = c.ExpectString("Ok, exiting.")
				require.NoError(t, err)
			},
			exitCode: 1,
		},
		{
			name: "run xata pull, answer Yes",
			args: []string{"pull"},
			procedure: func(t *testing.T, c *expect.Console) {
				_, err := c.ExpectString("Branch [test:new_feature] does not exist. Would you like to pull the schema from another branch?")
				require.NoError(t, err)

				_, err = c.SendLine("Y")
				require.NoError(t, err)

				_, err = c.ExpectString("From which branch should I pull the schema?")
				require.NoError(t, err)

				_, err = c.SendLine("main")
				require.NoError(t, err)

				_, err = c.ExpectString("Pulling schema from branch [test:main]")
				require.NoError(t, err)

				_, err = c.ExpectString("Your existing schema file has been backed up to xata/schema.json.bak")
				require.NoError(t, err)

				_, err = c.ExpectString("Pulled schema written to xata/schema.json")
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
