package cmd

import (
	"os"
	"testing"

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

	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "init", "--workspaceid", workspaceID)
	defer c.Close()

	_, err := c.ExpectString("New database name:")
	require.NoError(t, err)

	_, err = c.SendLine("test")
	require.NoError(t, err)

	_, err = c.ExpectString("Init done. Edit xata/schema.json to get started.")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)
}
