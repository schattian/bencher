package bencher

import (
	"fmt"
	"os"
)

const (
	ServerImage     = "ghcr.io/schattian/bencher:master"
	ServerRootPath  = "/bencher"
	ContainersLabel = "bencher"

	RunnerRootPath = "/bencher"

	db  = "db"
	pid = "pid"
)

var (
	// host paths
	HostRootPath       = fmt.Sprintf("%s/.bencher", os.Getenv("HOME"))
	HostVersionsPath   = fmt.Sprintf("%s/versions", HostRootPath)
	HostServerRootPath = fmt.Sprintf("%s/server", HostRootPath)
	HostDBFilename     = fmt.Sprintf("%s/%s", HostServerRootPath, db)
	HostPIDFilename    = fmt.Sprintf("%s/%s", HostServerRootPath, pid)

	// server paths
	ServerDBFilename  = fmt.Sprintf("%s/%s", ServerRootPath, db)
	ServerPIDFilename = fmt.Sprintf("%s/%s", ServerRootPath, pid)
)
