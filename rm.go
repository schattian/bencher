package main

import (
	"fmt"
	"os/exec"
	"strings"

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
		fmt.Println(cmd.Help())
		return 2
	}
	args, force := popFlagBoolean(args, "f")
	fmt.Println(force)
	if args[0] == "--all" || args[0] == "-all" {
		err := cmd.rmAllJobs()
		if err != nil {
			fmt.Printf("error occurred during rmAllJobs: %v", err)
			return 1
		}
		return 0
	}
	err := cmd.rmJobs(args...)
	if err != nil {
		if err != nil {
			fmt.Printf("error occurred during rmJobs: %v", err)
			return 1
		}
	}
	return 0
}

func (cmd *rmCmd) rmAllJobs() error {
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
	if err != nil {
		return errors.Wrap(err, "rmAllJobs")
	}
	err = rmAllFromSched(db)
	if err != nil {
		return errors.Wrap(err, "rmAllSched")
	}

	return nil
}

func (cmd *rmCmd) rmJobs(jobs ...string) error {
	for _, version := range jobs {
		execCmd := exec.Command("rm", "-rf", fmt.Sprintf("%s/%s/", bencher.HostVersionsPath, version))
		err := execCmd.Run()
		if err != nil {
			return errors.Wrap(err, "rm -rf")
		}
	}

	db, err := initDB()
	if err != nil {
		fmt.Printf("couldn't init db: %v", err)
		return errors.Wrap(err, "initDB")
	}
	defer db.Close()
	err = rmFromSched(db, jobs...)
	if err != nil {
		fmt.Printf("couldn't rm jobs from sched: %v", err)
		return errors.Wrap(err, "rmFromSched")
	}
	err = rmFromDB(db, jobs...)
	if err != nil {
		return errors.Wrap(err, "rmJobs")
	}
	return nil
}

func rmFromDB(db *bbolt.DB, versions ...string) error {
	return db.Update(func(tx *bbolt.Tx) error {
		bJobs := tx.Bucket(bencher.KeyJob)
		if bJobs == nil {
			return nil
		}
		return bJobs.ForEach(func(version, jobBytes []byte) error {
			if isInStrSl(string(version), versions) {
				err := bJobs.Delete(version)
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
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

func rmFromSched(db *bbolt.DB, versions ...string) error {
	return db.Update(func(tx *bbolt.Tx) error {
		bSched, err := tx.CreateBucketIfNotExists(bencher.KeySched)
		if err != nil {
			return err
		}
		sched := string(bSched.Get(bencher.KeySched))
		var newSched string
		for _, pendingVer := range strings.Split(sched, ",") {
			if !isInStrSl(pendingVer, versions) {
				newSched += fmt.Sprintf("%s,", pendingVer)
			}
		}
		return bSched.Put(bencher.KeySched, []byte(newSched))
	})
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
	return `remove versions`
}

func (cmd *rmCmd) Help() string {
	return `Usage: bencher rm [--all] <version1> <version2> <...>`
}
