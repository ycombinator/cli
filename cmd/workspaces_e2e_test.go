package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func createWorkspaceCommand(t *testing.T, config *TestConfig, configDir, name string) string {
	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "--json", "--nocolor", "workspaces", "create", name)
	defer c.Close()

	output, err := c.ExpectString("}")
	require.NoError(t, err)
	require.NotEmpty(t, output)

	var response struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(output), &response)
	require.NoError(t, err)

	require.Equal(t, name, response.Name)
	require.NotEmpty(t, response.ID)

	err = cmd.Wait()
	require.NoError(t, err)

	return response.ID
}

func deleteWorkspaceCommand(t *testing.T, config *TestConfig, configDir, workspaceID string) {
	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "workspaces", "delete", workspaceID)
	defer c.Close()

	err := cmd.Wait()
	require.NoError(t, err)
}

// createTestWorkspace logs in with the API key from the environmnent and creates
// a test workspace.
func createTestWorkspace(t *testing.T) (config TestConfig, configDir string, workspaceID string) {
	var err error
	config, err = GetTestConfigFromEnv()
	require.NoError(t, err)
	require.NotEmpty(t, config.TestKey)

	configDir, err = ioutil.TempDir("", "xata-config")
	require.NoError(t, err)
	t.Cleanup(func() {
		defer os.RemoveAll(configDir)
	})

	loginWithKey(t, &config, configDir)

	workspaceID = createWorkspaceCommand(t, &config, configDir, "test")

	t.Cleanup(func() {
		deleteWorkspaceCommand(t, &config, configDir, workspaceID)
	})

	return
}

func TestWorkspacesCreate(t *testing.T) {
	config, err := GetTestConfigFromEnv()
	require.NoError(t, err)
	require.NotEmpty(t, config.TestKey, "env variable 'TEST_API_KEY' must be present to test")

	configDir, err := ioutil.TempDir("", "xata-config")
	require.NoError(t, err)
	defer os.RemoveAll(configDir)

	loginWithKey(t, &config, configDir)

	workspaceID := createWorkspaceCommand(t, &config, configDir, "test")
	defer deleteWorkspaceCommand(t, &config, configDir, workspaceID)
}

func TestWorkspacesDeleteNotExistant(t *testing.T) {
	config, err := GetTestConfigFromEnv()
	require.NoError(t, err)
	require.NotEmpty(t, config.TestKey, "env variable 'TEST_API_KEY' must be present to test")

	configDir, err := ioutil.TempDir("", "xata-config")
	require.NoError(t, err)
	defer os.RemoveAll(configDir)

	loginWithKey(t, &config, configDir)

	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "workspaces", "delete", "test")
	defer c.Close()

	_, err = c.ExpectString("Auth error: no access to the workspace")
	require.NoError(t, err)
	_, err = c.ExpectString("For more information please see https://docs.xata.io/cli/getting-started")
	require.NoError(t, err)

	err = cmd.Wait()
	require.Error(t, err)
}
