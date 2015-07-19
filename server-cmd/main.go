package main

// Toy server useful when developing. In practice just use nginx, because
// all we're doing is serving up static content.

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func showHelpAndExit() {
	fmt.Println("commands:")
	fmt.Println("  serve root-dir   Run an HTTP server, with /files/* serving up root-dir/*")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		showHelpAndExit()
	}
	switch os.Args[1] {
	case "serve":
		if len(os.Args) < 3 {
			fmt.Printf("No root-dir specified\n")
			os.Exit(1)
		}
		root := os.Args[2]
		http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(root))))
		log.Fatal(http.ListenAndServe(":8080", nil))
	default:
		fmt.Printf("Unrecognized command '%v'\n", os.Args[1])
		os.Exit(1)
	}
}
