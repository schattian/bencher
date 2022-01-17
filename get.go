package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	docker *client.Client
}

func prepareGet() (cli.Command, error) {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	pruneContainers(context.Background(), docker)
	return &getCmd{docker: docker}, nil
}

func (cmd *getCmd) Run(args []string) int {
	db, err := initDB()
	if err != nil {
		fmt.Printf("err initDB: %v", err)
		return 1
	}
	defer db.Close()
	switch len(args) {
	case 0:
		err = errors.Wrap(cmd.printListJobs(db), "printListJobs")
	case 1:
		err = errors.Wrap(cmd.printJobDetail(db, args[0]), "printJobDetail")
	default:
		return cli.RunResultHelp
	}
	if err != nil {
		fmt.Printf("err %v", err)
		return 1
	}
	return 0
}

func (cmd *getCmd) printJobDetail(db *bbolt.DB, version string) error {
	j := &bencher.Job{}
	err := db.View(func(tx *bbolt.Tx) error {
		bJobs := tx.Bucket(bencher.KeyJob)
		if bJobs == nil {
			return nil
		}
		b := bJobs.Get([]byte(version))
		if b == nil {
			return nil
		}
		return json.Unmarshal(b, j)
	})
	if err != nil {
		return errors.Wrap(err, "db.View")
	}
	if j.Version == "" {
		v, err := getRunningVersion()
		if os.IsNotExist(err) {
			err = nil
		}
		if err != nil {
			return errors.Wrap(err, "getRunningVersion")
		}
		if v != version {
			fmt.Printf("job %s not found", version)
			return nil
		}
		j.Version = v
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

func (cmd *getCmd) printListJobs(db *bbolt.DB) error {
	jobs, err := listJobs(db)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 3, 3, 3, ' ', 0)
	fmt.Fprintln(w, "name\tstatus\t")
	for _, job := range jobs {
		fmt.Fprintf(w, "%s\t%s\t\n", job.Version, job.Status())
	}

	runningVer, err := getRunningVersion()
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return errors.Wrap(err, "getRunningVersion")
	}
	if runningVer != "" {
		fmt.Fprintf(w, "%s\t%s\t\n", runningVer, "running")
	}
	err = db.View(func(tx *bbolt.Tx) error {
		bSched := tx.Bucket(bencher.KeySched)
		if bSched == nil {
			return nil
		}
		sched := string(bSched.Get(bencher.KeySched))
		for i, pendingVer := range strings.Split(sched, ",") {
			if pendingVer == "" {
				continue
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
	return `get job(s) detail or print the jobs list`
}

func (cmd *getCmd) Help() string {
	return `Usage: bencher get [version]

Print details for the given version. In case no version is given, list all jobs. It's aliased with "ls"`
}
