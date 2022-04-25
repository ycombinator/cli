package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func RunHook(dir string, hook string) error {
	settings, err := ReadSettings(dir)

	if err != nil {
		return err
	}

	command := settings.Hooks[hook]
	if command == "" {
		return nil
	}

	cmd := buildCommand(command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("Error running %s hook. Tried to run the following command '%s': %w.", hook, command, err)
	}

	return nil
}

func buildCommand(command string) *exec.Cmd {
	// We need to run the command through a shell to support any kind of command, such as:
	// FOO=bar something --flag 2> /dev/null

	// SHELL is a standard environment varialbe in POSIX systems
	// See: https://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html
	shell := os.Getenv("SHELL")
	if shell == "" {
		// If SHELL is not defined, we probably are on windows, just in case we check that
		if runtime.GOOS == "windows" {
			// We default to powershell on windows
			shell = "pwsh"
		} else {
			// In any other case we default to bash, which is the most popular shell
			shell = "/bin/bash"
		}
	}

	// Shells can run files or inline commands. We want to run an inline command and for that we need
	// to pass the command with a flag.

	// The -c flag is standard in POSIX shells https://pubs.opengroup.org/onlinepubs/009695399/utilities/xcu_chap02.html
	flag := "-c"
	if shell == "pwsh" {
		// For powershell we need to use -Command
		flag = "-Command"
	}

	return exec.Command(shell, flag, command)
}
