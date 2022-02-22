package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xata/cli/client"
	"github.com/xata/cli/client/spec"
	"github.com/xata/cli/config"

	"github.com/TylerBrock/colorjson"
	"github.com/c-bata/go-prompt"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

var commandsSuggestions = []prompt.Suggest{
	{Text: "GET", Description: "GET request"},
	{Text: "POST", Description: "POST request"},
	{Text: "PUT", Description: "PUT request"},
	{Text: "DELETE", Description: "DELETE request"},
}

func commandCompleter(d prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(commandsSuggestions, d.GetWordBeforeCursor(), true)
}

func ShellCommand(c *cli.Context) error {
	workspaceID, err := getWorkspaceID(c)
	if err != nil {
		return err
	}

	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}
	xata, err := client.NewXataClient(apiKey, workspaceID)
	if err != nil {
		return fmt.Errorf("Error getting Xata client: %w", err)
	}

	completer := newCompleterEnv(xata)
	go completer.refreshDBCache(c.Context)

	if c.Bool("current") {
		dbName, _, branch, err := getDBNameAndBranch(c)
		if err != nil {
			return err
		}
		if len(dbName) > 0 && len(branch) > 0 {
			completer.prefix = fmt.Sprintf("/db/%s:%s", dbName, branch)
		}
	}

	executor := &executorEnv{
		Prettify:  !c.Bool("nopretty"),
		LightBG:   c.Bool("lightbg"),
		NoColor:   c.Bool("nocolor"),
		completer: completer,
		xata:      xata,
	}

	promptColor := prompt.Yellow
	if executor.LightBG {
		promptColor = prompt.Blue
	}
	if executor.NoColor {
		promptColor = prompt.DefaultColor
	}

	fmt.Println("xata interactive shell")
	fmt.Println("Please use `exit` or `Ctrl-D` to exit")
	p := prompt.New(
		executor.executor(c.Context),
		completer.completer(c.Context),
		prompt.OptionTitle("xata shell"),
		prompt.OptionPrefix(">>> "),
		prompt.OptionLivePrefix(completer.getLivePrefix),
		prompt.OptionInputTextColor(promptColor),
	)
	p.Run()
	return nil
}

type completerEnv struct {
	xata *spec.Client

	cacheMutex  sync.Mutex
	dbNames     []string
	branchNames map[string][]string
	tableNames  map[string]map[string][]string

	prefix string
}

func newCompleterEnv(xata *spec.Client) *completerEnv {
	return &completerEnv{
		xata:        xata,
		dbNames:     []string{},
		branchNames: map[string][]string{},
		tableNames:  map[string]map[string][]string{},
	}
}

func (env *completerEnv) getLivePrefix() (prefix string, useLivePrefix bool) {
	if len(env.prefix) == 0 {
		return "", false
	}
	return fmt.Sprintf("%s >> ", env.prefix), true
}

func (env *completerEnv) refreshDBCache(ctx context.Context) error {
	resp, err := env.xata.GetDatabaseList(ctx)
	if err != nil {
		return err
	}

	databases, err := spec.ParseGetDatabaseListResponse(resp)
	if err != nil {
		return err
	}

	dbNames := []string{}
	if databases.JSON200 == nil {
		return nil
	}

	for _, db := range *databases.JSON200.Databases {
		dbNames = append(dbNames, db.Name)
	}

	env.cacheMutex.Lock()
	defer env.cacheMutex.Unlock()
	env.dbNames = dbNames
	env.branchNames = map[string][]string{}
	env.tableNames = map[string]map[string][]string{}
	return nil
}

func (env *completerEnv) refreshTableNames(ctx context.Context, dbName, branchName string) error {
	resp, err := env.xata.GetBranchDetails(ctx, spec.DBBranchNameParam(fmt.Sprintf("%s:%s", dbName, branchName)))
	if err != nil {
		return err
	}

	branchDetails, err := spec.ParseGetBranchDetailsResponse(resp)
	if err != nil {
		return err
	}

	tableNames := []string{}
	if branchDetails.JSON200 == nil {
		return nil
	}

	for _, table := range branchDetails.JSON200.Schema.Tables {
		tableNames = append(tableNames, string(table.Name))
	}

	env.cacheMutex.Lock()
	defer env.cacheMutex.Unlock()
	_, exists := env.tableNames[dbName]
	if !exists {
		env.tableNames[dbName] = map[string][]string{}
	}
	env.tableNames[dbName][branchName] = tableNames
	return nil
}

func (env *completerEnv) refreshBranchNames(ctx context.Context, dbName string) error {
	resp, err := env.xata.GetBranchList(ctx, spec.DBNameParam(dbName))
	if err != nil {
		return err
	}

	branches, err := spec.ParseGetBranchListResponse(resp)
	if err != nil {
		return err
	}

	branchNames := []string{}
	if branches == nil || branches.JSON200 == nil {
		return nil
	}

	for _, branch := range branches.JSON200.Branches {
		branchNames = append(branchNames, branch.Name)
	}

	env.cacheMutex.Lock()
	defer env.cacheMutex.Unlock()
	env.branchNames[dbName] = branchNames
	return nil
}

func (env *completerEnv) baseAndTableCompleter(ctx context.Context, d prompt.Document, argument string, words []string) []prompt.Suggest {
	if len(words) == 2 {
		env.cacheMutex.Lock()
		defer env.cacheMutex.Unlock()
		suggests := []prompt.Suggest{}
		for _, name := range env.dbNames {
			suggests = append(suggests, prompt.Suggest{
				Text: fmt.Sprintf("%s/%s", words[0], name),
			})
		}
		if len(words[1]) == 0 {
			return suggests
		}
		return prompt.FilterHasPrefix(suggests, argument, true)
	}

	if len(words) == 3 {
		dbName := words[1]

		env.cacheMutex.Lock()
		defer env.cacheMutex.Unlock()
		branchNames, exists := env.branchNames[dbName]
		if !exists {
			go env.refreshBranchNames(ctx, dbName)
		} else {
			suggests := []prompt.Suggest{}
			for _, name := range branchNames {
				suggests = append(suggests, prompt.Suggest{
					Text: fmt.Sprintf("%s/%s/%s", words[0], dbName, name),
				})
			}
			if len(words[2]) == 0 {
				return suggests
			}
			return prompt.FilterHasPrefix(suggests, argument, true)
		}
	}

	if len(words) == 4 {
		dbName := words[1]
		branchName := words[2]

		env.cacheMutex.Lock()
		defer env.cacheMutex.Unlock()
		tableNames, exists := env.tableNames[dbName][branchName]
		if !exists {
			go env.refreshTableNames(ctx, dbName, branchName)
		} else {
			suggests := []prompt.Suggest{}
			for _, name := range tableNames {
				suggests = append(suggests, prompt.Suggest{
					Text: fmt.Sprintf("%s/%s/%s/%s", words[0], dbName, branchName, name),
				})
			}
			if len(words[3]) == 0 {
				return suggests
			}
			return prompt.FilterHasPrefix(suggests, argument, true)
		}
	}
	return []prompt.Suggest{}
}

func (env *completerEnv) getCompleter(ctx context.Context, d prompt.Document, argument string) []prompt.Suggest {
	words := strings.Split(argument, "/")

	suggests := []prompt.Suggest{
		{Text: "/", Description: "Get the existing bases."},
		{Text: "/_hello", Description: "Get server version information."},
		{Text: "/<dbname>", Description: "Operations on a database."},
		{Text: "/<dbname>/<table>", Description: "Operations on a table."},
	}

	if len(words) == 0 {
		return suggests
	}

	suggestions := prompt.FilterHasPrefix(suggests, argument, true)
	if len(suggestions) > 0 {
		return suggestions
	}

	if len(words) < 5 {
		return env.baseAndTableCompleter(ctx, d, argument, words)
	}

	if len(words) == 5 {
		head := filepath.Dir(argument)
		suggests := []prompt.Suggest{
			{Text: fmt.Sprintf("%s/_schema", head), Description: "Put schema to table."},
		}
		return prompt.FilterHasPrefix(suggests, argument, true)
	}

	return []prompt.Suggest{}
}

func (env *completerEnv) postCompleter(ctx context.Context, d prompt.Document, argument string) []prompt.Suggest {
	words := strings.Split(argument, "/")

	if len(argument) == 0 {
		return []prompt.Suggest{
			{Text: "/<dbname>", Description: "Operations on a database."},
			{Text: "/<dbname>/<table>", Description: "Operations on a table."},
		}
	}

	if argument == "/" || len(words) < 5 {
		return env.baseAndTableCompleter(ctx, d, argument, words)
	}

	return []prompt.Suggest{}
}

func (env *completerEnv) putCompleter(ctx context.Context, d prompt.Document, argument string) []prompt.Suggest {
	words := strings.Split(argument, "/")

	if len(argument) == 0 {
		return []prompt.Suggest{
			{Text: "/<dbname>", Description: "Operations on a database."},
			{Text: "/<dbname>/<table>", Description: "Operations on a table."},
		}
	}

	if argument == "/" || len(words) < 5 {
		return env.baseAndTableCompleter(ctx, d, argument, words)
	}

	if len(words) == 4 {
		head := filepath.Dir(argument)
		suggests := []prompt.Suggest{
			{Text: fmt.Sprintf("%s/_schema", head), Description: "Put schema to table."},
		}
		return prompt.FilterHasPrefix(suggests, argument, true)
	}

	return []prompt.Suggest{}
}

func (env *completerEnv) deleteCompleter(ctx context.Context, d prompt.Document, argument string) []prompt.Suggest {
	words := strings.Split(argument, "/")

	if len(argument) == 0 {
		return []prompt.Suggest{
			{Text: "/<dbname>", Description: "Operations on a database."},
			{Text: "/<dbname>/<table>", Description: "Operations on a table."},
		}
	}

	if argument == "/" || len(words) < 4 {
		return env.baseAndTableCompleter(ctx, d, argument, words)
	}

	return []prompt.Suggest{}
}

func (env *completerEnv) completer(ctx context.Context) prompt.Completer {
	return func(d prompt.Document) []prompt.Suggest {

		if d.TextBeforeCursor() == "" {
			return []prompt.Suggest{}
		}
		args := strings.Split(d.TextBeforeCursor(), " ")

		if len(args) == 1 {
			return commandCompleter(d)
		} else if len(args) == 2 {
			switch args[0] {
			case "GET":
				return env.getCompleter(ctx, d, args[1])
			case "POST":
				return env.postCompleter(ctx, d, args[1])
			case "PUT":
				return env.putCompleter(ctx, d, args[1])
			case "DELETE":
				return env.deleteCompleter(ctx, d, args[1])
			}
		}

		return []prompt.Suggest{}
	}
}

type executorEnv struct {
	Prettify  bool
	LightBG   bool
	NoColor   bool
	completer *completerEnv
	xata      *spec.Client
}

func (env *executorEnv) executor(ctx context.Context) prompt.Executor {
	return func(input string) {
		input = strings.TrimSpace(input)
		switch {
		case input == "":
			return
		case input == "quit" || input == "exit":
			fmt.Println("Bye!")
			os.Exit(0)
			return

		case strings.HasPrefix(input, "prefix"):
			words := strings.Split(input, " ")
			if len(words) > 2 {
				return
			}
			if words[0] != "prefix" {
				return
			}
			if len(words) == 1 {
				env.completer.prefix = ""
				return
			}
			env.completer.prefix = strings.TrimRight(words[1], "/ ")
			return
		}

		words := strings.Split(input, " ")
		if len(words) < 2 {
			return
		}
		method := words[0]
		urlPath := words[1]
		if len(env.completer.prefix) > 0 {
			urlPath = path.Join(env.completer.prefix, urlPath)
		}
		var arg *map[string]interface{}
		if len(words) > 2 {
			argument := strings.Join(words[2:], " ")
			var jsonArg map[string]interface{}
			err := json.Unmarshal([]byte(argument), &jsonArg)
			if err != nil {
				fmt.Printf("Argument must be JSON. Failed to unmarshal: %s\n", err)
				return
			}
			arg = &jsonArg
		}

		resp, err := request(ctx, env.xata, method, urlPath, arg)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading body: %s\n", err)
			return
		}

		if method == "POST" || method == "PUT" || method == "DELETE" {
			go env.completer.refreshDBCache(ctx)
		}

		if !env.Prettify {
			fmt.Printf("%s\n", bodyBytes)
			return
		}
		var response map[string]interface{}
		err = json.Unmarshal(bodyBytes, &response)
		if err != nil {
			fmt.Printf("%s\n", bodyBytes)
			return
		}
		if env.NoColor {
			s, err := json.MarshalIndent(response, "", "  ")
			if err != nil {
				fmt.Printf("%s\n", bodyBytes)
				return
			}
			fmt.Println(string(s))
			return
		}

		colorer := colorjson.NewFormatter()
		colorer.Indent = 2
		if env.LightBG {
			colorer.KeyColor = color.New(color.FgGreen)
			colorer.StringColor = color.New(color.FgBlack)
		} else {
			colorer.KeyColor = color.New(color.FgGreen)
			colorer.StringColor = color.New(color.FgWhite)
		}
		s, err := colorer.Marshal(response)
		if err != nil {
			fmt.Printf("%s\n", bodyBytes)
			return
		}

		fmt.Println(string(s))
	}
}

func request(ctx context.Context, client *spec.Client, method, urlPath string, body interface{}) (*http.Response, error) {
	bytesBody := []byte{}
	if body != nil {
		var err error
		bytesBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body to JSON: %w", err)
		}
	}

	reqURL, err := url.Parse(client.Server)
	if err != nil {
		return nil, err
	}

	pathAndParams, err := url.Parse(urlPath)
	if err != nil {
		return nil, fmt.Errorf("can't understand urlPath (%s): %w", urlPath, err)
	}

	reqURL = reqURL.ResolveReference(pathAndParams)

	reqURL.RawQuery = pathAndParams.RawQuery
	q := reqURL.Query()
	q.Add("_pretty", "true")
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), bytes.NewReader(bytesBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	err = applyEditors(ctx, client, req)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	return resp, nil
}

func applyEditors(ctx context.Context, c *spec.Client, req *http.Request) error {
	for _, r := range c.RequestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}
