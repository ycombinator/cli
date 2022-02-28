package cmd

import (
	"os"
	"testing"

	"github.com/Netflix/go-expect"
	"github.com/stretchr/testify/require"
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
