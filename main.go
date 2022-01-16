package main

import (
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/mitchellh/cli"
)

func main() {
	c := cli.NewCLI("app", "1.0.0")
	c.Args = os.Args[1:]

	c.Commands = map[string]cli.CommandFactory{
		"run":     prepareRun,
		"get":     prepareGet,
		"restore": prepareRestore,
		"rm":      prepareRm,
	}
	rand.Seed(time.Now().Unix())

	exitStatus, err := c.Run()
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(exitStatus)
}

var (
	defaultCmd = []string{"go", "test", "-bench=.", "-benchmem"}
)

const (
	//https://hub.docker.com/layers/golang/library/golang/alpine3.15/images/sha256-f28579af8a31c28fc180fb2e26c415bf6211a21fb9f3ed5e81bcdbf062c52893
	minDockerImage    = "golang@sha256:f28579af8a31c28fc180fb2e26c415bf6211a21fb9f3ed5e81bcdbf062c52893"
	defaultUnixSocket = "/var/run/docker.sock"
)
