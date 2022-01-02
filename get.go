package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/client"
	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
	"github.com/schattian/bencher/internal/bencher"
	"go.etcd.io/bbolt"
)

type getCmd struct {
	db     *bbolt.DB
	docker *client.Client
}

func prepareGet() (cli.Command, error) {
	db, err := initDB()
	if err != nil {
		return nil, err
	}
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &getCmd{db: db, docker: docker}, nil
}

func (cmd *getCmd) Run(args []string) int {
	var err error
	switch len(args) {
	case 0:
		err = cmd.runListJobs()
	case 1:
		err = cmd.runGetJobDetail(args[0])
	default:
		return 129
	}
	if err != nil {
		log.Fatal(err)
	}
	return 0
}

func (cmd *getCmd) runGetJobDetail(version string) error {
	j := &bencher.Job{}
	err := cmd.db.View(func(tx *bbolt.Tx) error {
		bJobs := tx.Bucket(bencher.KeyJob)
		if bJobs == nil {
			return nil
		}
		b := bJobs.Get([]byte(version))
		return json.Unmarshal(b, j)
	})
	if err != nil {
		return errors.Wrap(err, "db.View")
	}

	detail := fmt.Sprintf("name: %s\nstatus: %s", j.Version, j.Status())
	if j.Stdout != "" {
		detail = fmt.Sprintf("%s\noutput:\n\t%s", detail, strings.ReplaceAll(j.Stdout, "\n", "\n\t"))
	}
	if j.Stderr != "" {
		detail = fmt.Sprintf("%s\nerror:\n\t%s", detail, strings.ReplaceAll(j.Stderr, "\n", "\n\t"))
	}
	fmt.Println(detail)
	return nil
}

func (cmd *getCmd) runListJobs() error {
	jobs, err := listJobs(cmd.db)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 3, 3, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\t")
	for _, job := range jobs {
		fmt.Fprintf(w, "%s\t%s\t\n", job.Version, job.Status())
	}

	err = cmd.db.View(func(tx *bbolt.Tx) error {
		bSched := tx.Bucket(bencher.KeySched)
		if bSched == nil {
			return nil
		}
		sched := string(bSched.Get(bencher.KeySched))
		for i, pendingVer := range strings.Split(sched, ",") {
			if pendingVer == "" {
				return nil
			}
			fmt.Fprintf(w, "%s\t%s\t\n", pendingVer, fmt.Sprintf("scheduled at order #%d", i))
		}
		return nil
	})
	w.Flush()

	return nil
}

func listJobs(db *bbolt.DB) (jobs []*bencher.Job, err error) {
	err = db.View(func(tx *bbolt.Tx) error {
		bJobs := tx.Bucket(bencher.KeyJob)
		if bJobs == nil {
			return nil
		}
		return bJobs.ForEach(func(version, jobBytes []byte) error {
			j := &bencher.Job{}
			err := json.Unmarshal(jobBytes, j)
			if err != nil {
				return err
			}
			jobs = append(jobs, j)
			return nil
		})
	})
	return
}

func (cmd *getCmd) Synopsis() string {
	return `schedule a job`
}

func (cmd *getCmd) Help() string {
	return ``
}
