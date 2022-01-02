package bencher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

type Job struct {
	Stdout  string
	Stderr  string
	Version string
}

var KeyJob = []byte("jobs")

func (j *Job) Status() string {
	status := "pending"
	if j.Stdout != "" {
		status = "done"
	}
	if j.Stderr != "" {
		status = "errored"
	}
	return status
}

func (j *Job) Complete(ctx context.Context, db *bbolt.DB, docker *client.Client) error {
	wait, errCh := docker.ContainerWait(ctx, j.Version, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return errors.Wrap(err, "wait")
		}
	case <-wait:
		err := j.Collect(ctx, docker)
		if err != nil {
			return errors.Wrap(err, "collect")
		}
		err = j.Save(ctx, db)
		if err != nil {
			return errors.Wrap(err, "save")
		}
		err = j.Teardown(ctx, docker)
		if err != nil {
			return errors.Wrap(err, "teardown")
		}
		return nil
	}
	return nil
}

func (j *Job) Teardown(ctx context.Context, docker *client.Client) error {
	return docker.ContainerRemove(ctx, j.Version, types.ContainerRemoveOptions{})
}

func (j *Job) Save(ctx context.Context, db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(KeyJob)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		data, err := json.Marshal(j)
		if err != nil {
			return err
		}
		return b.Put([]byte(j.Version), data)
	})
}

// todo: collect in the meantime with follow and tail so it can be obtained through get
func (j *Job) Collect(ctx context.Context, docker *client.Client) error {
	out, err := docker.ContainerLogs(ctx, j.Version, types.ContainerLogsOptions{ShowStdout: true, Follow: true})
	if err != nil {
		return err
	}
	defer out.Close()
	results, errs := &bytes.Buffer{}, &bytes.Buffer{}
	_, err = stdcopy.StdCopy(results, errs, out)
	if err != nil {
		return err
	}
	j.Stdout, j.Stderr = results.String(), errs.String()
	return nil
}

func (j *Job) RunNow(ctx context.Context, db *bbolt.DB, docker *client.Client) error {
	err := docker.ContainerStart(ctx, j.Version, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrap(err, "start")
	}
	err = j.Complete(ctx, db, docker)
	if err != nil {
		return errors.Wrap(err, "complete")
	}
	return nil
}
