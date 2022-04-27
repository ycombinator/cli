package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"github.com/xataio/cli/filesystem"
)

const DirPerms = 0700

func InstallCodegen(c *cli.Context, dir string) error {
	install := c.Bool("ts-codegen")
	if !install {
		if !isInteractive(c) {
			return nil
		}
		prompt := &survey.Confirm{
			Message: "Do you want to install the TypeScript SDK and code generator?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &install); err != nil {
			return err
		}
	}

	if !install {
		return nil
	}

	pm := "npm"
	pmi := "install"
	pmr := "npx"

	if exists, _ := filesystem.FileExists("yarn.lock"); exists {
		pm = "yarn"
		pmi = "add"
		pmr = "yarn"
	}

	if path, _ := exec.LookPath(pm); path == "" {
		return fmt.Errorf("looks like %s is not installed or is not in the PATH. This made impossible to install the code generator", pm)
	}

	if err := execPackageManager(pm, pmi, "@xata.io/client"); err != nil {
		return fmt.Errorf("the command to install @xata.io/client failed: %w", err)
	}

	if err := execPackageManager(pm, pmi, "@xata.io/codegen", "-D"); err != nil {
		return fmt.Errorf("the command to install @xata.io/codegen failed: %w", err)
	}

	settings, err := ReadSettings(dir)
	if err != nil {
		return err
	}

	if settings.Hooks == nil {
		settings.Hooks = map[string]string{}
	}
	settings.Hooks["build"] = fmt.Sprintf("%s xata-codegen %s -o src/xata.ts", pmr, dir)

	if err = writeSettings(dir, *settings); err != nil {
		return err
	}

	return nil
}

func execPackageManager(npm string, arg ...string) error {
	cmd := exec.Command(npm, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
