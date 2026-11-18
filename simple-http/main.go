package main

import (
	"fmt"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("./"))
	http.Handle("/", fs)
	
	port := ":8080"
	fmt.Println("Starting server at", port)
	http.ListenAndServe(port, nil)
}
