package main

// Toy server useful when developing

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
