package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/client"
	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
	"github.com/schattian/bencher/internal/bencher"
	"go.etcd.io/bbolt"
	"golang.org/x/perf/benchstat"
)

type cmpCmd struct {
	docker *client.Client
}

func prepareCmp() (cli.Command, error) {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	pruneContainers(context.Background(), docker)
	return &cmpCmd{docker: docker}, nil
}

func (cmd *cmpCmd) Run(args []string) int {
	if len(args) < 2 {
		return cli.RunResultHelp
	}
	db, err := initDB()
	if err != nil {
		fmt.Printf("err initDB: %v", err)
		return 1
	}
	defer db.Close()
	err = cmd.cmp(db, args...)
	if err != nil {
		log.Fatal(err)
	}
	return 0
}

func getJobs(db *bbolt.DB, versions ...string) (jobs []*bencher.Job, err error) {
	err = db.View(func(tx *bbolt.Tx) error {
		bJobs := tx.Bucket(bencher.KeyJob)
		if bJobs == nil {
			return nil
		}
		for _, v := range versions {
			b := bJobs.Get([]byte(v))
			if b == nil {
				continue
			}
			j := &bencher.Job{}
			json.Unmarshal(b, j)
			jobs = append(jobs, j)
		}
		return nil
	})
	return
}

func (cmd *cmpCmd) cmp(db *bbolt.DB, versions ...string) error {
	jobs, err := getJobs(db, versions...)
	if err != nil {
		return errors.Wrap(err, "getJobs")
	}
	c := &benchstat.Collection{}
	for _, job := range jobs {
		if job.Status() != "done" {
			continue
		}
		fmt.Println(job.Stdout)
		err := c.AddFile(job.Version, bytes.NewBufferString(job.Stdout))
		if err != nil {
			return errors.Wrap(err, "benchstat.Collection.AddFile: %v")
		}
	}
	var buf bytes.Buffer
	benchstat.FormatText(&buf, c.Tables())
	_, err = os.Stdout.Write(buf.Bytes())
	return errors.Wrap(err, "os.Stdout.Write")
}

func (cmd *cmpCmd) Synopsis() string {
	return `compare two or more versions`
}

func (cmd *cmpCmd) Help() string {
	return `Usage: bencher cmp <version1> <version2> [version3] [...]

Compare two or more versions with benchstat`
}
