# GIFSpot
A distributed CRUD app for GIFs written in Go!

## Features:
- Concurrent backend
- Frontend uses [Iris](https://github.com/kataras/iris)
- Failure Detector
- “Ticket Storage” strategy for thread-safe data store

`targets.txt`  and `data.txt` are provided so [Vegeta](https://github.com/tsenart/vegeta) can be used for load testing

## Work in progress
- Implement Raft algorithm on backend so data is replicated across backends

