package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QLight-dev/LearnHTTPServersGo/ch5/e4/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Chirp struct {
	ID        uuid.NullUUID `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Body      string        `json:"body"`
	UserID    uuid.UUID     `json:"user_id"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}
type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries      *database.Queries
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

	err := godotenv.Load("../../.env")
	if err != nil {
		panic(fmt.Sprintf("Failed to load .env file \n Error Message: %v", err))
	}

	db, err := sql.Open("postgres", os.Getenv("DB_URL"))

	if err != nil {
		panic(fmt.Sprintf("Failed to open database \n Error Message: %v", err))
	}

	mux := http.NewServeMux()
	var cfg apiConfig
	cfg.dbQueries = database.New(db)

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
		// send forbidden request status if platform is not local dev enviorment so prevent dangrous sql statements being used in prod
		if os.Getenv("PLATFORM") != "dev" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		cfg.fileServerHits.Store(0)
		if err = cfg.dbQueries.DeleteAllUsers(req.Context()); err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, req *http.Request) {
		type requestShape struct {
			Email string `json:"email"`
		}

		var body requestShape
		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		dbUser, err := cfg.dbQueries.CreateUser(req.Context(), body.Email)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		resData := User{
			ID:        dbUser.ID.UUID,
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: dbUser.UpdatedAt,
			Email:     body.Email,
		}

		res, err := json.Marshal(resData)
		if err != nil {
			w.WriteHeader(400)
			return
		}

		w.WriteHeader(201)
		io.WriteString(w, string(res))
	})

	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, req *http.Request) {
		type expectedShape struct {
			Body   string `json:"body"`
			UserID string `json:"user_id"`
		}

		decoder := json.NewDecoder(req.Body)
		var body expectedShape
		err := decoder.Decode(&body)

		if err != nil {
			res, _ := json.Marshal(errorResponse{Err: "Inocrrect shape"})
			w.WriteHeader(http.StatusBadRequest)
			w.Write(res)
			return
		}

		if len(body.Body) >= 140 {
			res, _ := json.Marshal(errorResponse{Err: "Chirp is too long"})
			w.WriteHeader(http.StatusBadRequest)
			w.Write(res)
			return
		}

		cleanedBody := removeProfaneWords(body.Body)

		parsedUserID, err := uuid.Parse(body.UserID)
		if err != nil {
			res, _ := json.Marshal(errorResponse{Err: "UserID is not a valid UUID"})
			w.WriteHeader(http.StatusBadRequest)
			w.Write(res)
			return
		}

		chirp, err := cfg.dbQueries.CreateChirp(req.Context(), database.CreateChirpParams{Body: cleanedBody, UserID: parsedUserID})
		if err != nil {
			res, _ := json.Marshal(errorResponse{Err: err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			w.Write(res)
			return
		}

		resData := Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		}

		var resBuf bytes.Buffer
		encoder := json.NewEncoder(&resBuf)
		encoder.Encode(resData)

		w.WriteHeader(201)
		w.Write(resBuf.Bytes())
	})

	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
