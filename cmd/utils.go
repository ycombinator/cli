package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/tidwall/pretty"
	"github.com/xataio/cli/client"
	"github.com/xataio/cli/client/spec"
	"github.com/xataio/cli/config"

	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
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

func printJSON(c *cli.Context, bodyBytes []byte) error {
	if len(bodyBytes) == 0 {
		return nil
	}

	if c.Bool("nocolor") {
		fmt.Println(string(pretty.Pretty(bodyBytes)))
		return nil
	}

	var response map[string]interface{}
	err := json.Unmarshal(bodyBytes, &response)
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
	return nil
}

type BasicResponse interface {
	StatusCode() int
	Status() string
}

func printResponse(c *cli.Context, resp BasicResponse, bodyBytes []byte, err error, printer func() error) error {
	if err != nil {
		return fmt.Errorf("Sending request: %s\n", err)
	}
	if resp.StatusCode()/100 != 2 {
		if resp.StatusCode() == http.StatusUnauthorized {
			return ErrorUnauthorized{message: getMessage(bodyBytes)}
		}
		return fmt.Errorf("%s: %s", resp.Status(), getMessage(bodyBytes))
	}
	if c.Bool("json") {
		return printJSON(c, bodyBytes)
	}
	if printer == nil {
		return nil
	}
	return printer()
}

func printTable(headers []string, table [][]interface{}) {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	tabHeaders := make([]interface{}, len(headers)*2-1)
	for i := 0; i < len(headers); i++ {
		tabHeaders[i*2] = headers[i]
		if i > 0 {
			tabHeaders[i*2-1] = "\t"
		}
	}
	fmt.Fprintln(w, tabHeaders...)

	for i := 0; i < len(table); i++ {
		row := table[i]
		tabRow := make([]interface{}, len(row)*2-1)
		for n := 0; n < len(row); n++ {
			tabRow[n*2] = printableValue(row[n])
			if n > 0 {
				tabRow[n*2-1] = "\t"
			}
		}
		fmt.Fprintln(w, tabRow...)
	}
	w.Flush()
}

func printableValue(value interface{}) interface{} {
	switch v := value.(type) {
	case spec.DateTime:
		return time.Time(v).UTC().String()
	default:
		return value
	}
}

func isInteractiveWithReason(c *cli.Context) (bool, string) {
	if c.Bool("no-input") {
		return false, "--no-input is being used"
	}
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return false, "the current terminal is not interactive"
	}
	return true, ""
}

func isInteractive(c *cli.Context) bool {
	interactive, _ := isInteractiveWithReason(c)
	return interactive
}

func errorIfNotInteractive(c *cli.Context, variable string) error {
	_, reason := isInteractiveWithReason(c)
	if reason == "" {
		return nil
	}
	return fmt.Errorf("In order to proceed a value for %s is required but a value was not passed as an argument and interactivity is disabbled because %s", variable, reason)
}
