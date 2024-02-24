# fgamanager (WIP)

![fgamanager_shot.png](fgamanager_shot.png)

Alternative to the official [OpenFGA](https://openfga.dev/) [playground](https://play.fga.dev/) that has some limitations.

`fgamanager` is a text based UI built with Golang's [tview](https://github.com/rivo/tview). The goal is to have a table oriented navigation with a simple search to find tuples of interest. Users will be allowed to mark them for deletion or create new ones. 

Contributions welcome!

## Running it

It takes only two env variables at the moment `API_URL` and `STORE_ID`. The database will be stored in the current directory.
```shell
API_URL=https://myopenfga:8080 STORE_ID=03HME1444HSEY9022AENH1YYKFJ go run .
```


## How it works

So far I've made a risk decision to se FGA's [canges endpoint](https://openfga.dev/api/service#/Relationship%20Tuples/ReadChanges) to replicate tuples locally to a SQLite. SQLite can be shared and saved to a cheap storage like S3 or GCS, then shared if needed.

It keeps the last state only, meaning all changes are applied locally but not kept, reducing the data that will be needed in general. This is not tested against billions of rows, which might be challenging, but for ordinary setups with millions of rows, this should be stable enough.

