package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("OK"))
	})

	err := http.ListenAndServe(":3333", nil)
	fmt.Println(err)
}
