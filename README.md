# Bencher

Bencher is a command line utility that runs and versions go benchmarks in an isolated and scheduled environment.

You can schedule multiple different benchmarks without thinking on your local environment

[gif of run and run within subdirs]


You can get the current list of jobs and inspect the results for completed jobs 

[gif of get list and details]


You can benchstat across two or more versions

[gif of cmp]


Each build is versioned and saved independently so you can restore them even if you discarded those changes before, specially handy when trying several different approaches

[gif of restore]


Finally, you can remove unwanted versions (or even stop the one running). Notice that this removes the local copy of the version, so you cannot restore it afterwards

[gif of rm]


## Motivation

Trying several different approaches when optimizing was a pain for me, since you need to wait until the latest benchmark finished to start a new one (otherwise, you end up sharing resources, which makes benchmarks more unreliable).
Also, I wanted a way to try those approaches without the overhead of managing versions with a VCS (sometimes, I just want to try multiple little changes in specific combinations), so I added the `restore` command for this.
Finally, I wanted to allocate a certain amount of resources to the benchmarks, so they don't bother while I'm doing other stuff (personally, I use a separated docker host, see [Recommendations](#recommendations)).

## Requirements

It runs using the docker api, which implies that you must be a docker client, but you don't need to have the docker CLI installed nor host the docker server (i.e: if you've the socket, it's fine).
Also, the benchmarks need to be part of a go module, although it doesn't matter where you want to run them (e.g: if you run them in a subdir).

## Installation

```
go install github.com/schattian/bencher
```
## Recommendations

More reliable results are obtained if you run them in a physically separated docker host (e.g: a raspi, a homeserver, a cloud instance).
To do so, you can follow these steps:

1. Ensure that you have ssh access to the machine.
2. Set `DOCKER_HOST=ssh://username@host:port`.

The reason behind that is resource allocation. If you run/stop something while running the benchmarks, these could be affected indirectly (and, considering that most 
use a browser and slack/spotify, this is not negligible).
