package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middlewareFileServerHitsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, req)
	})
}

type errorResponse struct {
	Err string `json:"error"`
}

func removeProfaneWords(text string) string {
	splitString := strings.Split(text, " ")

	for wordIndex, word := range splitString {
		if strings.ToLower(word) == "kerfuffle" || strings.ToLower(word) == "sharbert" || strings.ToLower(word) == "fornax" {
			splitString[wordIndex] = "****"
		}
	}

	return strings.Join(splitString, " ")
}

func main() {
	mux := http.NewServeMux()
	var cfg apiConfig
	mux.Handle("/app/", http.StripPrefix("/app", cfg.middlewareFileServerHitsInc(http.FileServer(http.Dir(".")))))

	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileServerHits.Load())
	})

	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, req *http.Request) {
		cfg.fileServerHits.Store(0)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, req *http.Request) {
		type expectedShape struct {
			Body string `json:"body"`
		}

		expectedShapeDecoder := json.NewDecoder(req.Body)
		var body expectedShape
		err := expectedShapeDecoder.Decode(&body)

		if err != nil {
			res, _ := json.Marshal(errorResponse{Err: "Something went wrong"})
			io.WriteString(w, string(res))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(body.Body) >= 140 {
			res, _ := json.Marshal(errorResponse{Err: "Chirp is too long"})
			io.WriteString(w, string(res))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		fmt.Fprintf(w, `{"cleaned_body":"%v"}`, removeProfaneWords(body.Body))
		w.WriteHeader(http.StatusOK)
	})

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
