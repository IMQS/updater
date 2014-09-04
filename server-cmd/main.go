package main

/*
This is really just a static file server. At some point it might be worthwhile to
integrate bsdiff into this, so that a client can request a diff between one file
and another, for example:

https://deploy.imqs.co.za/updates/diff/c629cee85a65c4b818221038835d74791151727c-4b37e3919462a3153d7527013e020c08f42df700

In this example, the client is requesting the diff between two files (c629... to 4b37...).
The server could pre-emptively fill up static files in the 'diff' directory, for common upgrade paths.
In all practicality, this would probably cover 99% of our server upgrades, since most upgrades
will be on a very predictable track.
*/

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: server root-dir")
		os.Exit(1)
	}
	root := os.Args[1]
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(root))))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
