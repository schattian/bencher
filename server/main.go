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
	db, err := initDB()
	if err != nil {
		log.Fatal(err)
	}
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}
	for {
		c := cli.NewCLI("app", "1.0.0")
		c.Args = os.Args[1:]
		c.Commands = map[string]cli.CommandFactory{
			"sched": prepareSched(db, docker),
		}
		rand.Seed(time.Now().Unix())
		exitStatus, err := c.Run()
		if err != nil {
			log.Println(err)
		}
		os.Exit(exitStatus)
	}
}

func initDB() (*bbolt.DB, error) {
	return bbolt.Open(bencher.ServerDBFilename, 0600, bbolt.DefaultOptions)
}

func pidLock() (func() error, error) {
	f, err := os.OpenFile(bencher.ServerPIDFilename, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	return func() error { return os.Remove(f.Name()) }, nil
}

var schedKey = []byte("sched")

type runCmd struct {
	db *bbolt.DB
}

func run(ctx context.Context, db *bbolt.DB, docker *client.Client, j *bencher.Job) error {
	// TODO: release pid immediately after initializing server. Do it. What happens with multithreading?
	pidUnlock, err := pidLock()
	if os.IsExist(err) {
		sched(db, j)
		return nil
		// todo corner race: lock in case pid is going to be released and runNext is going to run then
	}
	if err != nil {
		return errors.Wrap(err, "pidLock")
	}
	err = j.RunNow(ctx, db, docker)
	pidUnlock()
	if err != nil {
		return errors.Wrap(err, "runNow")
	}
	err = runNext(ctx, db, docker, j)
	if err != nil {
		return errors.Wrap(err, "runNext")
	}
	return nil
}

func runNext(ctx context.Context, db *bbolt.DB, docker *client.Client, j *bencher.Job) error {
	nextJ, err := lookupNext(db)
	if err != nil {
		return errors.Wrap(err, "lookupNext")
	}
	if nextJ == nil {
		return nil
	}
	defer unsched(db, nextJ.Version)
	return errors.Wrap(run(ctx, db, docker, j), "run")
}

func sched(db *bbolt.DB, job *bencher.Job) error {
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
