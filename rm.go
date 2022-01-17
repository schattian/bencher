package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
	"github.com/schattian/bencher/internal/bencher"
	"go.etcd.io/bbolt"
)

type rmCmd struct{}

func prepareRm() (cli.Command, error) {
	return &rmCmd{}, nil
}

func (cmd *rmCmd) Run(args []string) int {
	if len(args) < 1 {
		return cli.RunResultHelp
	}
	args, force := popFlagBoolean(args, "f")
	if args[0] == "--all" || args[0] == "-all" {
		err := cmd.rmAllJobs(force)
		if err != nil {
			fmt.Printf("err rmAllJobs: %v", err)
			return 1
		}
		return 0
	}
	err := cmd.rmJobs(force, args...)
	if err != nil {
		if err != nil {
			fmt.Printf("err during rmJobs: %v", err)
			return 1
		}
	}
	return 0
}

func (cmd *rmCmd) rmAllJobs(force bool) error {
	if force {
		err := stopRunningJob(context.Background(), func(string) bool { return true })
		if err != nil {
			return errors.Wrap(err, "stopRunningJob")
		}
	}

	execCmd := exec.Command("rm", "-rf", bencher.HostVersionsPath)
	err := execCmd.Run()
	if err != nil {
		fmt.Printf("rm -rf failed: %v\n", err)
		return errors.Wrap(err, "rm -rf")
	}
	db, err := initDB()
	if err != nil {
		fmt.Printf("couldn't init db: %v", err)
		return errors.Wrap(err, "initDB")
	}
	defer db.Close()
	err = rmAllFromDB(db)
	if errors.Is(err, bbolt.ErrBucketNotFound) {
		err = nil
	}
	if err != nil {
		return errors.Wrap(err, "rmAllFromDB")
	}
	err = rmAllFromSched(db)
	if errors.Is(err, bbolt.ErrBucketNotFound) {
		err = nil
	}
	if err != nil {
		return errors.Wrap(err, "rmAllFromSched")
	}

	return nil
}

func (cmd *rmCmd) rmJobs(force bool, jobs ...string) error {
	for _, version := range jobs {
		execCmd := exec.Command("rm", "-rf", fmt.Sprintf("%s/%s/", bencher.HostVersionsPath, version))
		err := execCmd.Run()
		if err != nil {
			return errors.Wrap(err, "rm -rf")
		}
	}
	db, err := initDB()
	if err != nil {
		return errors.Wrap(err, "initDB")
	}
	defer db.Close()
	jobs, err = rmFromSched(db, jobs...)
	if err != nil {
		return errors.Wrap(err, "rmFromSched")
	}
	jobs, err = rmFromDB(db, jobs...)
	if err != nil {
		return errors.Wrap(err, "rmJobs")
	}
	if force {
		for i, job := range jobs {
			err = stopRunningJob(context.Background(), func(version string) bool { return version == job })
			if err != nil {
				return errors.Wrapf(err, "stopRunningJob[%d]", i)
			}
		}
	}
	return nil
}

func stopRunningJob(ctx context.Context, cond func(version string) bool) error {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return errors.Wrap(err, "docker.NewClientWithOpts")
	}
	defer docker.Close()

	version, err := getRunningVersion()
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !cond(version) {
		return nil
	}
	return docker.ContainerRemove(ctx, version, types.ContainerRemoveOptions{Force: true})
}

func getRunningVersion() (string, error) {
	version, err := os.ReadFile(bencher.HostPIDFilename)
	if err != nil {
		return "", err
	}
	return string(version), nil
}

func rmFromDB(db *bbolt.DB, versions ...string) (rest []string, err error) {
	var delJobs []string
	err = db.Update(func(tx *bbolt.Tx) error {
		bJobs := tx.Bucket(bencher.KeyJob)
		if bJobs == nil {
			return nil
		}
		return bJobs.ForEach(func(version, jobBytes []byte) error {
			if strVer := string(version); isInStrSl(strVer, versions) {
				delJobs = append(delJobs, strVer)
				return bJobs.Delete(version)
			}
			return nil
		})
	})
	if err != nil {
		return
	}
	rest = diffStrSl(versions, delJobs)
	return
}

func rmAllFromDB(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(bencher.KeyJob)
	})
}

func rmAllFromSched(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(bencher.KeySched)
	})
}

func rmFromSched(db *bbolt.DB, versions ...string) (rest []string, err error) {
	err = db.Update(func(tx *bbolt.Tx) error {
		bSched, err := tx.CreateBucketIfNotExists(bencher.KeySched)
		if err != nil {
			return err
		}
		sched := string(bSched.Get(bencher.KeySched))
		var newSched string
		var newSchedSl []string
		for _, pendingVer := range strings.Split(sched, ",") {
			if !isInStrSl(pendingVer, versions) {
				newSchedSl = append(newSchedSl, pendingVer)
				newSched += fmt.Sprintf("%s,", pendingVer)
			}
		}
		rest = diffStrSl(versions, newSchedSl)
		return bSched.Put(bencher.KeySched, []byte(newSched))
	})
	return
}

func diffStrSl(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func isInStrSl(s string, sl []string) bool {
	for _, ss := range sl {
		if s == ss {
			return true
		}
	}
	return false
}

func (cmd *rmCmd) Synopsis() string {
	return `remove version(s)`
}

func (cmd *rmCmd) Help() string {
	return `Usage: bencher rm [--all] [-f] <version1> [version2] [...]
	
Remove the specified version(s). If [--all] is given, delete all the versions 
If [-f] flag is given, include removing running versions 
`
}
