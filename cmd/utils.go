package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"

	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
	"github.com/tidwall/pretty"
	"github.com/urfave/cli/v2"
)

func getDirectoryName() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return path.Base(dir), nil
}

func checkWorkspacesResponse(workspaces *spec.GetWorkspacesListResponse) error {
	if workspaces.JSON401 != nil {
		return ErrorUnauthorized{message: workspaces.JSON401.Message}
	}
	if workspaces.JSON400 != nil {
		return fmt.Errorf("Error getting workspaces: %s", workspaces.JSON400.Message)
	}
	if workspaces.JSON404 != nil {
		return fmt.Errorf("Error getting workspaces: %s", workspaces.JSON404.Message)
	}

	if workspaces.StatusCode() != http.StatusOK {
		return fmt.Errorf("Error getting workspaces: %s", workspaces.Status())
	}
	if workspaces.JSON200 == nil {
		return fmt.Errorf("Error getting workspaces: 200 OK unexpected response body")
	}
	return nil
}

type Workspace struct {
	ID   spec.WorkspaceID `json:"id"`
	Name string           `json:"name"`
	Slug string           `json:"slug"`
}

func getWorkspaces(c *cli.Context) ([]Workspace, error) {
	apiKey, err := config.APIKey(c)
	if err != nil {
		return nil, err
	}
	client, err := client.NewXataClientWithResponses(apiKey, "")
	if err != nil {
		return nil, err
	}

	workspaces, err := client.GetWorkspacesListWithResponse(c.Context)
	if err != nil {
		return nil, err
	}
	if err = checkWorkspacesResponse(workspaces); err != nil {
		return nil, err
	}

	res := make([]Workspace, 0, len(workspaces.JSON200.Workspaces))
	for _, workspace := range workspaces.JSON200.Workspaces {
		res = append(res, Workspace{workspace.Id, workspace.Name, workspace.Slug})
	}
	return res, nil
}

func getDBs(c *cli.Context, workspaceID string) ([]string, error) {
	apiKey, err := config.APIKey(c)
	if err != nil {
		return nil, err
	}
	client, err := client.NewXataClientWithResponses(apiKey, workspaceID)
	if err != nil {
		return nil, err
	}

	dbs, err := client.GetDatabaseListWithResponse(c.Context)
	if err != nil {
		return nil, err
	}
	if dbs.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("error getting databases: %s", dbs.Status())
	}

	if dbs.JSON200.Databases == nil {
		return []string{}, nil
	}

	res := make([]string, 0, len(*dbs.JSON200.Databases))
	for _, db := range *dbs.JSON200.Databases {
		res = append(res, string(db.Name))
	}
	return res, nil
}

func gitGetRepoAndBranchName() (repo, branch string, err error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", "", err
	}

	buffer := bufio.NewReader(bytes.NewReader(out))
	directory, err := buffer.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	repo = filepath.Base(directory)
	branch, err = buffer.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	return strings.Trim(repo, " \n"), strings.Trim(branch, " \n"), nil
}

func GitGetLastSHA() (sha string, err error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(string(out), "\n"), nil
}

func GitHasLocalChanges(filename string) (bool, error) {
	out, err := exec.Command("git", "-c", "color.status=false", "status", "-s", filename).Output()
	if err != nil {
		return false, err
	}
	if len(out) >= 2 && (string(out[0:2]) == "??" || string(out[0:2]) == " M" || string(out[0:2]) == "M ") {
		return true, nil
	}
	return false, nil
}

func getMessage(bytes []byte) string {
	var resp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(bytes, &resp); err != nil {
		return string(bytes)
	}
	return resp.Message
}

func printResponse(c *cli.Context, resp *http.Response, err error) error {
	if err != nil {
		return fmt.Errorf("Sending request: %s\n", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusUnauthorized {
			return ErrorUnauthorized{message: getMessage(bodyBytes)}
		}
		return fmt.Errorf("Error from server: status %d: %s", resp.StatusCode, getMessage(bodyBytes))
	}

	if len(bodyBytes) == 0 {
		return nil
	}

	if c.Bool("nocolor") {
		fmt.Println(string(pretty.Pretty(bodyBytes)))
	} else {
		var response map[string]interface{}
		err = json.Unmarshal(bodyBytes, &response)
		if err != nil {
			fmt.Printf("%s\n", bodyBytes)
			return err
		}

		colorer := colorjson.NewFormatter()
		colorer.Indent = 2
		if c.Bool("lightbg") {
			colorer.KeyColor = color.New(color.FgGreen)
			colorer.StringColor = color.New(color.FgBlack)
		} else {
			colorer.KeyColor = color.New(color.FgGreen)
			colorer.StringColor = color.New(color.FgWhite)
		}
		s, err := colorer.Marshal(response)
		if err != nil {
			fmt.Printf("%s\n", bodyBytes)
			return err
		}

		fmt.Println(string(s))
	}
	return nil
}
