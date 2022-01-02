package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
	"github.com/schattian/bencher/internal/bencher"
	"go.etcd.io/bbolt"
)

func main() {
	c := cli.NewCLI("app", "1.0.0")
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"run": prepareRun,
		"get": prepareGet,
	}
	rand.Seed(time.Now().Unix())

	exitStatus, err := c.Run()
	if err != nil {
		log.Println(err)
	}

	os.Exit(exitStatus)
}

var (
	defaultCmd = []string{"go", "test", "-bench=.", "-benchmem"}
)

const (
	minDockerImage    = "golang:alpine"
	defaultUnixSocket = "/var/run/docker.sock"
)

type runCmd struct {
	docker *client.Client
}

func (cmd *runCmd) Run(args []string) int {
	var version string
	args, version = popNameFlag(args)
	if version == "" {
		version = namesgenerator.GetRandomName(0)
		fmt.Printf("version name not given, using `%s`. To give a version name use the `-name` flag\n", version)
	}
	ctx := context.Background()

	err := cmd.prepareRuntime(ctx, version, args)
	if err != nil {
		log.Fatal(err)
	}
	err = runServerCmd(ctx, cmd.docker, []string{"sched", version}, os.Getenv("BENCHER_DEBUG") != "")
	if err != nil {
		log.Fatal(err)
	}
	return 0
}

func popFlag(args []string, flagName string) ([]string, string) {
	pivot := -2
	singleFlagName := fmt.Sprintf("-%s", flagName)
	doubleFlagName := fmt.Sprintf("-%s", singleFlagName)
	for i, arg := range args {
		if arg == singleFlagName || arg == doubleFlagName {
			pivot = i
		}
		if i == pivot+1 {
			return append(args[:i-1], args[i+1:]...), arg
		}
		if strings.HasPrefix(arg, fmt.Sprintf("%s=", doubleFlagName)) {
			return append(args[:i], args[i+1:]...), arg[7:]
		}
		if strings.HasPrefix(arg, fmt.Sprintf("%s=", singleFlagName)) {
			return append(args[:i], args[i+1:]...), arg[6:]
		}
	}

	return args, ""
}

func popNameFlag(args []string) ([]string, string) {
	return popFlag(args, "name")
}

func pruneContainers(ctx context.Context, docker *client.Client) error {
	args := filters.NewArgs(filters.Arg("label", bencher.ContainersLabel))
	_, err := docker.ContainersPrune(ctx, args)
	if err != nil {
		return err
	}
	return nil
}

func (cmd *runCmd) prepareRuntime(ctx context.Context, version string, forward []string) error {
	err := pruneContainers(ctx, cmd.docker)
	if err != nil {
		return err
	}

	err = os.MkdirAll(bencher.HostServerRootPath, os.ModePerm)
	if err != nil {
		return err
	}
	root, err := getModPath()
	if err != nil {
		return errors.Wrap(err, "getModPath")
	}
	versionPath := filepath.Join(bencher.HostVersionsPath, filepath.Base(root), version)
	err = os.MkdirAll(versionPath, os.ModePerm)
	if err != nil {
		return err
	}

	execCmd := exec.Command("rsync", "-av", "--progress", root+"/", versionPath, "--exclude", ".git")
	err = execCmd.Run()
	if err != nil {
		return err
	}

	log.Println("preparing go vendor...")
	execCmd = exec.Command("go", "mod", "vendor")
	execCmd.Dir = versionPath
	err = execCmd.Run()
	if err != nil {
		return err
	}

	log.Println("preparing docker container...")
	r, err := cmd.docker.ImagePull(ctx, minDockerImage, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(io.Discard, r)
	if err != nil {
		return err
	}
	if len(forward) == 0 {
		fmt.Printf("command not given, using the default one (`go test -bench=. -benchmem`). To give a command just use args\n")
		forward = defaultCmd
	}
	err = createContainer(ctx, cmd.docker, version, versionPath, forward)
	if err != nil {
		return err
	}

	if err != nil {
		return errors.Wrap(err, "job.init")
	}
	return nil
}

func initDB() (*bbolt.DB, error) {
	return bbolt.Open(bencher.HostDBFilename, 0600, bbolt.DefaultOptions)
}

func runServerCmd(ctx context.Context, docker *client.Client, cmd []string, debug bool) error {
	containerName := fmt.Sprintf("%s_%s", bencher.ContainersLabel, namesgenerator.GetRandomName(0))
	r, err := docker.ContainerCreate(
		ctx,
		&container.Config{
			Image:      bencher.ServerImage,
			Env:        []string{"CGO_ENABLED=0"},
			Labels:     map[string]string{bencher.ContainersLabel: "server"},
			WorkingDir: bencher.ServerRootPath,
			Volumes: map[string]struct{}{
				defaultUnixSocket:          {},
				bencher.HostServerRootPath: {},
			},
			Cmd: cmd,
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: defaultUnixSocket,
					Target: defaultUnixSocket,
				},
				{
					Type:   mount.TypeBind,
					Source: bencher.HostServerRootPath,
					Target: bencher.ServerRootPath,
				},
			},
		},
		nil,
		nil,
		containerName,
	)
	if isContainerExists(err) {
		return runServerCmd(ctx, docker, cmd, debug)
	}
	if err != nil {
		return errors.Wrap(err, "create")
	}
	err = docker.ContainerStart(ctx, r.ID, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrap(err, "start")
	}
	if debug {
		wait, errCh := docker.ContainerWait(ctx, r.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			return errors.Wrap(err, "wait")
		case <-wait:
		}
	}
	return nil
}

func isContainerExists(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), "Conflict. The container name")
}

// docker.ContainerRemove(ctx, r.ID, types.ContainerRemoveOptions{})
func createContainer(ctx context.Context, docker *client.Client, version, versionPath string, cmd []string) error {
	_, err := docker.ContainerCreate(
		ctx,
		&container.Config{
			Image:      minDockerImage,
			Env:        []string{"CGO_ENABLED=0"}, // TODO
			Labels:     map[string]string{bencher.ContainersLabel: "runner"},
			WorkingDir: bencher.RunnerRootPath,
			Entrypoint: strslice.StrSlice{""},
			Volumes: map[string]struct{}{
				versionPath: {},
			},
			Cmd: cmd,
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: versionPath,
					Target: bencher.RunnerRootPath,
				},
			},
		},
		nil,
		nil,
		version,
	)

	if err != nil {
		return err
	}
	return nil
}

func prepareRun() (cli.Command, error) {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &runCmd{docker: docker}, nil
}

func (cmd *runCmd) Synopsis() string {
	return `// TODO:`
}

func (cmd *runCmd) Help() string {
	return "TODO"
}

// This code is heavily inspired from golang/go/src/go/build/build.go
func getModPath() (string, error) {
	parent, err := os.Getwd()
	if err != nil {
		return "", errors.New("not using modules")
	}
	for {
		if f, err := os.Open(filepath.Join(parent, "go.mod")); err == nil {
			buf := make([]byte, 100)
			_, err := f.Read(buf)
			f.Close()
			if err == nil || err == io.EOF {
				return parent, nil
			}
		}
		d := filepath.Dir(parent)
		if len(d) >= len(parent) {
			return "", errors.New("not using modules")
		}
		parent = d
	}
}
