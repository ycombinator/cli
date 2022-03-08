package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xataio/cli/filesystem"

	"github.com/urfave/cli/v2"
)

// ArgKey is the cmd argument used to configure the config dir path
const ArgKey = "configdir"

// env var to read the API key from
const APIKeyEnv = "XATA_API_KEY"

const FilePerms = 0600

const DirPerms = 0700

// Config dir to use, with the following precedence:
//
// 1. --configdir arg
// 2. $XATA_CONFIG_DIR env var
// 2. $XDG_CONFIG_HOME/xata if $XDG_CONFIG_HOME is defined
// 3. %APP_DATA%/Xata CLI folder (windows only)
// 4. ~/.config/xata
func ConfigDir(c *cli.Context) string {
	var path string
	if p := c.String(ArgKey); p != "" {
		path = p
	} else if p := os.Getenv("XATA_CONFIG_DIR"); p != "" {
		path = p
	} else if p := os.Getenv("XDG_CONFIG_HOME"); p != "" {
		path = filepath.Join(p, "xata")
	} else if p := os.Getenv("APP_DATA"); runtime.GOOS == "windows" && p != "" {
		path = filepath.Join(p, "Xata CLI")
	} else {
		p, _ := os.UserHomeDir()
		path = filepath.Join(p, ".config", "xata")
	}

	return path
}

// keyFile file stores the API key to access Xata API, located under `ConfigDir`/key
func keyFile(c *cli.Context) string {
	return filepath.Join(ConfigDir(c), "key")
}

// APIKeyInEnv returns true if API key is overridden by XATA_API_KEY env var
func APIKeyInEnv() bool {
	return apiKeyFromEnv() != ""
}

// APIKey reads and returns the configured API key
func APIKey(c *cli.Context) (string, error) {
	// is API key overridden from env?
	if k := apiKeyFromEnv(); k != "" {
		return k, nil
	}

	// normal flow, if user called `xata auth login`
	logged, err := LoggedIn(c)
	if err != nil {
		return "", err
	}
	if !logged {
		return "", errors.New("Xata CLI is not configured, please run `xata auth login`")
	}

	return readKeyFile(keyFile(c))
}

func StoreAPIKey(c *cli.Context, key string) error {
	err := os.MkdirAll(ConfigDir(c), DirPerms)
	if err != nil {
		return err
	}
	return os.WriteFile(keyFile(c), []byte(key), FilePerms)
}

func RemoveAPIKey(c *cli.Context) error {
	return os.Remove(keyFile(c))
}

// LoggedIn returns true if the cli is authenticated
func LoggedIn(c *cli.Context) (bool, error) {
	if APIKeyInEnv() {
		return true, nil
	}

	exists, err := filesystem.FileExists(keyFile(c))
	if err != nil {
		return false, err
	}

	return exists, nil
}

func readKeyFile(path string) (string, error) {
	// TODO fail if key file permissions are incorrect?
	keyBytes, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading keyfile: %w", err)
	}
	return strings.Trim(string(keyBytes), "\n "), nil
}

func apiKeyFromEnv() string {
	return os.Getenv(APIKeyEnv)
}
