package main

import (
	"fmt"
	"os/exec"

	"github.com/mitchellh/cli"
	"github.com/schattian/bencher/internal/bencher"
)

type restoreCmd struct{}

func prepareRestore() (cli.Command, error) {
	return &restoreCmd{}, nil
}

func (cmd *restoreCmd) Run(args []string) int {
	if len(args) != 2 {
		return cli.RunResultHelp
	}
	version, dst := args[0], args[1]
	execCmd := exec.Command("rsync", "-a", "--exclude", "vendor", fmt.Sprintf("%s/%s/", bencher.HostVersionsPath, version), dst)
	err := execCmd.Run()
	if err != nil {
		fmt.Printf("err rsync: %v\n", err)
		return 1
	}
	return 0
}

func (cmd *restoreCmd) Synopsis() string {
	return `restore the local copy of the given version to the given dir`
}

func (cmd *restoreCmd) Help() string {
	return `Usage: bencher restore <version> <destination>

You can either restore the version on a new dir or to the repo you've been working on by pointing <destination> to its root path`
}
