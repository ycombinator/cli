package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/kr/pty"
	"github.com/stretchr/testify/require"
	"github.com/xata/cli/internal/envcfg"
)

type TestConfig struct {
	TestKey        string `env:"TEST_API_KEY"`
	TestBinaryPath string `env:"TEST_XATA_PATH"`
}

const defaultXataCommand = "../xata"

func GetTestConfigFromEnv() (config TestConfig, err error) {
	err = envcfg.ReadEnv([]string{"../.env", "../.env.local"})
	if err != nil {
		return TestConfig{}, err
	}

	config.TestKey = os.Getenv("TEST_API_KEY")
	config.TestBinaryPath = os.Getenv("TEST_XATA_PATH")
	if config.TestBinaryPath == "" {
		config.TestBinaryPath = defaultXataCommand
	}
	config.TestBinaryPath, err = filepath.Abs(config.TestBinaryPath)
	if err != nil {
		return TestConfig{}, err
	}
	return
}

// NewVT10XConsole returns a new expect.Console that multiplexes the
// Stdin/Stdout to a VT10X terminal, allowing Console to interact with an
// application sending ANSI escape sequences.
func NewVT10XConsole(opts ...expect.ConsoleOpt) (*expect.Console, error) {
	ptm, pts, err := pty.Open()
	if err != nil {
		return nil, err
	}

	term := vt10x.New(vt10x.WithWriter(pts))

	c, err := expect.NewConsole(append(opts, expect.WithStdin(ptm), expect.WithStdout(term), expect.WithCloser(pts, ptm))...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func TestAuthLoginCommand(t *testing.T) {
	config, err := GetTestConfigFromEnv()
	require.NoError(t, err)
	require.NotEmpty(t, config.TestKey, "env variable 'TEST_API_KEY' must be present to test")

	configDir, err := ioutil.TempDir("", "xata-config")
	require.NoError(t, err)
	defer os.RemoveAll(configDir)

	tests := []struct {
		name      string
		procedure func(t *testing.T, c *expect.Console)
		exitCode  int
	}{
		{
			name: "try an invalid API key",
			procedure: func(t *testing.T, c *expect.Console) {
				_, err = c.ExpectString("Introduce your API key:")
				require.NoError(t, err)
				_, err = c.SendLine("invalid_key")
				require.NoError(t, err)

				_, err = c.Expect(
					expect.String("Checking access to the API...Auth error: Invalid API key"),
					expect.WithTimeout(5*time.Second))
				require.NoError(t, err)

				_, err = c.ExpectString("For more information please see https://docs.xata.io/cli/getting-started")
				require.NoError(t, err)
			},
			exitCode: 1,
		},
		{
			name: "login with valid API key",
			procedure: func(t *testing.T, c *expect.Console) {
				_, err = c.ExpectString("Introduce your API key:")
				require.NoError(t, err)
				_, err = c.SendLine(config.TestKey)
				require.NoError(t, err)

				_, err = c.Expect(
					expect.String("Checking access to the API...OK"),
					expect.WithTimeout(5*time.Second))
				require.NoError(t, err)

				_, err = c.ExpectString("All set! you can now start using xata")
				require.NoError(t, err)
			},
			exitCode: 0,
		},
		{
			name: "login again, should ask for a confirmation",
			procedure: func(t *testing.T, c *expect.Console) {
				_, err = c.ExpectString("Authentication is already configured, do you want to override it?")
				require.NoError(t, err)
				_, err = c.SendLine("y")
				require.NoError(t, err)

				_, err = c.ExpectString("Introduce your API key:")
				require.NoError(t, err)
				_, err = c.SendLine(config.TestKey)
				require.NoError(t, err)

				_, err = c.Expect(
					expect.String("Checking access to the API...OK"),
					expect.WithTimeout(5*time.Second))
				require.NoError(t, err)

				_, err = c.ExpectString("All set! you can now start using xata")
				require.NoError(t, err)
			},
			exitCode: 0,
		},
		{
			name: "answer No this time",
			procedure: func(t *testing.T, c *expect.Console) {
				_, err = c.ExpectString("Authentication is already configured, do you want to override it?")
				require.NoError(t, err)
				_, err = c.SendLine("N")
				require.NoError(t, err)

				_, err = c.ExpectString("No")
				require.NoError(t, err)
			},
			exitCode: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := NewVT10XConsole(
				expect.WithDefaultTimeout(1 * time.Second),
				// expect.WithStdout(os.Stdout),
			)
			require.NoError(t, err)
			defer c.Close()

			cmd := exec.Command(
				config.TestBinaryPath,
				fmt.Sprintf("--configdir=%s", configDir),
				"auth", "login")
			cmd.Stdin = c.Tty()
			cmd.Stdout = c.Tty()
			cmd.Stderr = c.Tty()

			err = cmd.Start()
			require.NoError(t, err)

			test.procedure(t, c)

			err = c.Close()
			require.NoError(t, err)
			err = cmd.Wait()
			require.Equal(t, test.exitCode, exitCodeFromError(t, err))
		})
	}
}

func exitCodeFromError(t *testing.T, err error) int {
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			require.NoError(t, err)
		}
	}
	return exitCode
}

func startCommand(t *testing.T, configDir string, binaryPath string, args ...string) (*expect.Console, *exec.Cmd) {
	c, err := NewVT10XConsole(
		expect.WithDefaultTimeout(5 * time.Second),
		// expect.WithStdout(os.Stdout),
	)
	require.NoError(t, err)

	args = append([]string{
		fmt.Sprintf("--configdir=%s", configDir),
	}, args...)

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	err = cmd.Start()
	require.NoError(t, err)

	return c, cmd
}

func loginWithKey(t *testing.T, config *TestConfig, configDir string) {
	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "auth", "login")
	defer c.Close()

	_, err := c.ExpectString("Introduce your API key:")
	require.NoError(t, err)
	_, err = c.SendLine(config.TestKey)
	require.NoError(t, err)

	_, err = c.Expect(
		expect.String("Checking access to the API...OK"),
		expect.WithTimeout(15*time.Second))
	require.NoError(t, err)

	_, err = c.ExpectString("All set! you can now start using xata")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)
}

func TestAuthStatus(t *testing.T) {
	config, err := GetTestConfigFromEnv()
	require.NoError(t, err)
	require.NotEmpty(t, config.TestKey, "env variable 'TEST_API_KEY' must be present to test")

	configDir, err := ioutil.TempDir("", "xata-config")
	require.NoError(t, err)
	defer os.RemoveAll(configDir)

	c, cmd := startCommand(t, configDir, config.TestBinaryPath, "auth", "status")

	_, err = c.ExpectString("You are not logged in, run `xata auth login` first")
	require.NoError(t, err)

	err = cmd.Wait()
	require.Equal(t, 1, exitCodeFromError(t, err))
	err = c.Close()
	require.NoError(t, err)

	loginWithKey(t, &config, configDir)

	c, cmd = startCommand(t, configDir, config.TestBinaryPath, "auth", "status")
	defer c.Close()
	_, err = c.ExpectString("Client is logged in")
	require.NoError(t, err)

	_, err = c.ExpectString("Checking access to the API...OK")
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)

}
