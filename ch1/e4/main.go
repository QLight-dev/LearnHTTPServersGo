package main

import "net/http"

type apiHandler struct{}

func (handler apiHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
