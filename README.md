# fgamanager (WIP)

[![asciicast](https://asciinema.org/a/642929.svg)](https://asciinema.org/a/642929)

> [!IMPORTANT]
> This is a experimental work in progress. Low test coverage and more playing with the whole setup to test
> its applicability.


Alternative to the official [OpenFGA](https://openfga.dev/) [playground](https://play.fga.dev/) that has some limitations.

`fgamanager` is a text based UI built with Golang's [tview](https://github.com/rivo/tview). The goal is to have a table oriented navigation with a simple search to find tuples of interest. Users will be allowed to mark them for deletion or create new ones. 

Contributions welcome!

## Running it

It takes only two env variables at the moment `API_URL` and `STORE_ID`. The database will be stored in the current directory.
```shell
go run . -a https://myopenfga:8080 -s 03HME1444HSEY9022AENH1YYKFJ 
```


## How it works

So far I've made a risk decision to se FGA's [canges endpoint](https://openfga.dev/api/service#/Relationship%20Tuples/ReadChanges) to replicate tuples locally to a SQLite. SQLite can be shared and saved to a cheap storage like S3 or GCS, then shared if needed.

It keeps the last state only, meaning all changes are applied locally but not kept, reducing the data that will be needed in general. This is not tested against billions of rows, which might be challenging, but for ordinary setups with millions of rows, this should be stable enough.

## High tuple volume
`fgamanger` was used with more than 1.3Mi tuples with no hiccups. This is possible employing [tview's virtual tables](https://github.com/rivo/tview/wiki/VirtualTable).
