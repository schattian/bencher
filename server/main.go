package main

/* RUNNING IS NOT DONE BY SV
api:
1. list jobs w/status
2. push job
3. pop job
*/

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
	"github.com/schattian/bencher/internal/bencher"
	"go.etcd.io/bbolt"
)

// func recreateJob(docker *client.Client, version string) (*job, error) {
// 	// docker.ContainerInspect()
// }

// type code int
// const (
// 	E_SCHED = 1
// )
// type bencherErr struct {
// 	code code
// 	err  error
// }

func main() {
	c := cli.NewCLI("app", "1.0.0")
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"sched": prepareSched,
	}
	rand.Seed(time.Now().Unix())
	exitStatus, err := c.Run()
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(exitStatus)
}

func initDB() (*bbolt.DB, error) {
	return bbolt.Open(bencher.ServerDBFilename, 0600, bbolt.DefaultOptions)
}

func pidLock() (func() error, error) {
	f, err := os.OpenFile(bencher.ServerPIDFilename, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	return func() error { return os.Remove(f.Name()) }, nil
}

var schedKey = []byte("sched")

type runCmd struct{}

func run(ctx context.Context, j *bencher.Job) error {
	pidUnlock, err := pidLock()
	if os.IsExist(err) {
		log.Printf("scheduling %s", j.Version)
		sched(j)
		return nil
		// todo corner race: lock in case pid is going to be released and runNext is going to run then
	}
	if err != nil {
		return errors.Wrap(err, "pidLock")
	}

	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer docker.Close()

	log.Printf("running %s", j.Version)
	err = j.RunNow(ctx, initDB, docker)
	pidUnlock()
	if err != nil {
		return errors.Wrap(err, "runNow")
	}
	err = runNext(ctx, j)
	if err != nil {
		return errors.Wrap(err, "runNext")
	}
	return nil
}

func runNext(ctx context.Context, j *bencher.Job) error {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer docker.Close()

	nextJ, err := lookupNext()
	if err != nil {
		return errors.Wrap(err, "lookupNext")
	}
	if nextJ == nil {
		return nil
	}
	err = unsched(nextJ.Version)
	if err != nil {
		return errors.Wrap(err, "unsched")
	}
	return errors.Wrap(run(ctx, nextJ), "run")
}

func sched(job *bencher.Job) error {
	db, err := initDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bencher.KeySched)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return b.Put(bencher.KeySched, append(b.Get(bencher.KeySched), []byte(job.Version+",")...))
	})
}

func implode(docker *client.Client) error {
	return docker.ContainerRemove(context.Background(), bencher.ContainersLabel, types.ContainerRemoveOptions{})
}
