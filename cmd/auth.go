package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"

	"github.com/xataio/cli/client"
	"github.com/xataio/cli/config"
)

func GetAuthSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "login",
			Usage:  "Authenticate with Xata",
			Action: login,
		},
		{
			Name:   "logout",
			Usage:  "Logout from Xata",
			Action: logout,
		},
		{
			Name:   "status",
			Usage:  "Check status of the auth settings",
			Action: loginStatus,
		},
	}
}

func login(c *cli.Context) error {
	if config.APIKeyInEnv() {
		return fmt.Errorf("Cannot configure auth if `%s` env var is present, it will be used instead", config.APIKeyEnv)
	}
	if interactive, reason := isInteractiveWithReason(c); !interactive {
		return fmt.Errorf("The login command is interactive but %s", reason)
	}

	configured, err := config.LoggedIn(c)
	if err != nil {
		return err
	}

	if configured {
		override := false
		prompt := &survey.Confirm{
			Message: "Authentication is already configured, do you want to override it?",
		}
		err = survey.AskOne(prompt, &override)
		if err != nil {
			return err
		}
		if !override {
			return nil
		}
	}

	var apiKey string
	prompt := &survey.Password{
		Message: "Introduce your API key:",
		Help:    "You can generate a new API key at https://app.xata.io. You can learn more about API keys on our documentation site: https://docs.xata.io/concepts/api-keys",
	}
	err = survey.AskOne(prompt, &apiKey)
	if err != nil {
		return err
	}

	// test the key actually works
	err = verifyAPIKeyValid(c.Context, apiKey)
	if err != nil {
		return err
	}

	// Store the new key
	err = config.StoreAPIKey(c, apiKey)
	if err != nil {
		return fmt.Errorf("Error saving the API key: %w", err)
	}

	fmt.Println("All set! you can now start using xata")
	return nil
}

func logout(c *cli.Context) error {
	if config.APIKeyInEnv() {
		return fmt.Errorf("Cannot configure auth if `%s` env var is present, it will be used instead", config.APIKeyEnv)
	}

	configured, err := config.LoggedIn(c)
	if err != nil {
		return err
	}

	if !configured {
		fmt.Println("You are not logged in")
	}

	logout := true
	if isInteractive(c) {
		prompt := &survey.Confirm{
			Message: "Are you sure you want to logout of Xata?",
		}
		err = survey.AskOne(prompt, &logout)
		if err != nil {
			return err
		}
	}
	if logout {
		err = config.RemoveAPIKey(c)
		if err != nil {
			return err
		}

		fmt.Println("Logged out correctly")
	}

	return nil
}

func loginStatus(c *cli.Context) error {
	if config.APIKeyInEnv() {
		fmt.Printf("Client API key configured through `%s` env var.\n", config.APIKeyEnv)
	} else {
		configured, err := config.LoggedIn(c)
		if err != nil {
			return err
		}

		if !configured {
			return errors.New("You are not logged in, run `xata auth login` first")
		}

		fmt.Println("Client is logged in")
	}

	apiKey, err := config.APIKey(c)
	if err != nil {
		return err
	}

	return verifyAPIKeyValid(c.Context, apiKey)
}

func verifyAPIKeyValid(ctx context.Context, apiKey string) error {
	// test the key actually works
	fmt.Printf("Checking access to the API...")
	client, err := client.NewXataClient(apiKey, "")
	if err != nil {
		return err
	}

	resp, err := client.GetWorkspacesList(ctx)
	if err != nil {
		return fmt.Errorf("Error accessing the API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrorUnauthorized{message: getMessageFromHTTPResponse(resp)}
	}
	if resp.StatusCode >= 299 {
		return fmt.Errorf("error: %s\n", resp.Status)
	}
	fmt.Println("OK")
	return nil
}

func getMessageFromHTTPResponse(resp *http.Response) string {
	if resp == nil {
		return ""
	}

	if resp.Body == nil {
		return ""
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var response struct {
		Message string `json:"message"`
	}
	err = json.Unmarshal(body, &response)
	if err != nil || len(response.Message) == 0 {
		return string(body)
	}

	return response.Message
}
