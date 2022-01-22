package main

import (
	"bytes"
	"context"
	"log"

	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
	"github.com/schattian/bencher/internal/bencher"
	"go.etcd.io/bbolt"
)

type schedCmd struct{}

func prepareSched() (cli.Command, error) {
	return &schedCmd{}, nil
}

func (cmd *schedCmd) Run(args []string) int {
	if len(args) == 0 {
		return 128
	}
	j := &bencher.Job{Version: args[0]}
	err := run(context.Background(), j)
	if err != nil {
		log.Fatal(err)
	}
	return 0
}

func lookupNext() (*bencher.Job, error) {
	var nextVersion string
	db, err := initDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bencher.KeySched)
		if err != nil {
			return errors.Wrap(err, "CreateBucket")
		}
		v := b.Get(bencher.KeySched)
		i := bytes.IndexByte(v, ',')
		if i <= 0 {
			return nil
		}
		nextVersion = string(v[:i])
		return nil
	})
	if err != nil {
		return nil, err
	}
	if nextVersion == "" {
		return nil, nil
	}
	j := &bencher.Job{Version: nextVersion}
	return j, nil
}

func unsched(version string) error {
	db, err := initDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bencher.KeySched)
		return b.Put(bencher.KeySched, b.Get(bencher.KeySched)[len(version)+1:])
	})
}

func (cmd *schedCmd) Synopsis() string {
	return `schedule a job`
}

func (cmd *schedCmd) Help() string {
	return ``
}
