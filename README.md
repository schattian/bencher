# Bencher

Bencher is a command line utility that runs go benchmarks in an isolated, scheduled environment. The idea is to have reproducible, versioned and comparable builds.

The schedule is there to guarantee that you can run just one benchmark at the time, so there's no shared resources between them and you can ensure that more or less the same resources were allocated for all of them. 

Each build is versioned and saved independently, and you can restore them even if you discarded those changes afterwards.



## Requirements

It runs using the docker api, which implies that you must be a docker client, but you don't need to have the docker CLI installed nor host the docker server (if you have the socket, it's fine).
Also, the benchmarks need to be part of a go module.

## Recommendations

The best results are obtained if you run them in a physically separated docker server (e.g: a raspi, a homeserver, a cloud instance).
To do so, you can follow these steps:
1. Ensure that you have ssh access to the machine.
2. Set `DOCKER_HOST=ssh://username@host:port`.

The reason behind that is, again, resource allocation. If you run/stop something while running the benchmarks, these could be affected indirectly (and, considering that most 
use a browser and slack/spotify, this is not negligible).
