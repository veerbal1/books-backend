package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

type Book struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

type CreateBookInput struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

type UpdateBookInput struct {
	Title  *string `json:"title"`
	Author *string `json:"author"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}

type SuccessResponse struct {
	Data any `json:"data"`
}

func seedBooks() []Book {
	return []Book{
		{
			ID:     1,
			Title:  "Rich Dad, Poor Dad",
			Author: "Japanese",
		},
		{
			ID:     2,
			Title:  "Harry Potter",
			Author: "American",
		},
	}
}

var books []Book = seedBooks()

const maxRequestBodySize = 64 << 10

type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

type ListResponse struct {
	Data       []Book     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

func limitRequestBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
}

func isBodyTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func parsePagination(r *http.Request) (int, int, error) {
	limit := 10
	offset := 0

	limitText := r.URL.Query().Get("limit")
	if limitText != "" {
		parsedLimit, err := strconv.Atoi(limitText)
		if err != nil || parsedLimit < 1 || parsedLimit > 100 {
			return 0, 0, fmt.Errorf("invalid limit")
		}
		limit = parsedLimit
	}

	offsetText := r.URL.Query().Get("offset")
	if offsetText != "" {
		parsedOffset, err := strconv.Atoi(offsetText)
		if err != nil || parsedOffset < 0 {
			return 0, 0, fmt.Errorf("invalid offset")
		}
		offset = parsedOffset
	}

	return limit, offset, nil
}

func parseSort(r *http.Request) (string, string, error) {
	sortField := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	if sortField == "" {
		sortField = "id"
	}
	if order == "" {
		order = "asc"
	}

	if sortField != "id" && sortField != "title" && sortField != "author" {
		return "", "", fmt.Errorf("invalid sort field")
	}

	if order != "asc" && order != "desc" {
		return "", "", fmt.Errorf("invalid sort order")
	}

	return sortField, order, nil
}

func bookHandler(w http.ResponseWriter, r *http.Request) {
	author := r.URL.Query().Get("author")
	title := r.URL.Query().Get("title")
	limit, offset, err := parsePagination(r)

	if err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid pagination",
		)
		return
	}

	sortField, order, err := parseSort(r)
	if err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid sorting",
		)
		return
	}

	result := []Book{}
	for _, book := range books {
		matchesAuthor := author == "" || strings.EqualFold(book.Author, author)
		matchesTitle := title == "" ||
			strings.Contains(strings.ToLower(book.Title), strings.ToLower(title))
		if matchesAuthor && matchesTitle {
			result = append(result, book)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		left := result[i]
		right := result[j]

		if sortField == "id" {
			if order == "asc" {
				return left.ID < right.ID
			}
			return left.ID > right.ID
		}

		leftValue := left.Title
		rightValue := right.Title

		if sortField == "author" {
			leftValue = left.Author
			rightValue = right.Author
		}

		leftValue = strings.ToLower(leftValue)
		rightValue = strings.ToLower(rightValue)

		if leftValue == rightValue {
			return left.ID < right.ID
		}

		if order == "asc" {
			return leftValue < rightValue
		}
		return leftValue > rightValue
	})

	total := len(result)

	page := []Book{}
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = result[offset:end]
	}

	hasMore := offset+len(page) < total

	res := ListResponse{
		Data: page,
		Pagination: Pagination{
			Limit:   limit,
			Offset:  offset,
			Total:   total,
			HasMore: hasMore,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: ErrorObject{Code: code, Message: message}})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	res := SuccessResponse{
		Data: map[string]string{"status": "ok"},
	}
	json.NewEncoder(w).Encode(res)
}

func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	res := SuccessResponse{
		Data: map[string]string{"message": "Welcome to the jungle."},
	}

	json.NewEncoder(w).Encode(res)
}

func getBookByIDHandler(store BookStore) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
			return
		}
		if idInt <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
			return
		}
		book, err := store.GetBookByID(r.Context(), idInt)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "book_not_found", "Request book not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		res := SuccessResponse{
			Data: book,
		}
		json.NewEncoder(w).Encode(res)
	})
}

func updateBookHandler(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		writeJSONError(
			w,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json",
		)
		return
	}

	limitRequestBody(w, r)

	id := r.PathValue("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}
	if idInt <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}
	for index, book := range books {
		if book.ID == idInt {
			var input UpdateBookInput
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&input)
			if err != nil {
				if isBodyTooLarge(err) {
					writeJSONError(
						w,
						http.StatusRequestEntityTooLarge,
						"body_too_large",
						"Request body is too large",
					)
					return
				}
				writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
				return
			}
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
				return
			}

			if input.Title != nil {
				trimmed := strings.TrimSpace(*input.Title)
				if trimmed == "" {
					writeJSONError(w, http.StatusBadRequest, "invalid_values", "Invalid values")
					return
				}
				*input.Title = trimmed
			}
			if input.Author != nil {
				trimmed := strings.TrimSpace(*input.Author)
				if trimmed == "" {
					writeJSONError(w, http.StatusBadRequest, "invalid_values", "Invalid values")
					return
				}
				*input.Author = trimmed
			}
			if input.Title == nil && input.Author == nil {
				writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
				return
			}

			if input.Title != nil {
				books[index].Title = *input.Title
			}
			if input.Author != nil {
				books[index].Author = *input.Author
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			res := SuccessResponse{
				Data: books[index],
			}
			json.NewEncoder(w).Encode(res)
			return
		}
	}
	writeJSONError(w, http.StatusNotFound, "book_not_found", "Request book not found")
}

func deleteBookHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil || idInt <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	for index, book := range books {
		if book.ID == idInt {
			books = append(books[:index], books[index+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	writeJSONError(w, http.StatusNotFound, "book_not_found", "Request book not found")
}

func isJSONContentType(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	return mediaType == "application/json"
}

func createBookHandler(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		writeJSONError(
			w,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json",
		)
		return
	}
	limitRequestBody(w, r)

	var input CreateBookInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err != nil {
		if isBodyTooLarge(err) {
			writeJSONError(
				w,
				http.StatusRequestEntityTooLarge,
				"body_too_large",
				"Request body is too large",
			)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}

	if strings.TrimSpace(input.Author) == "" || strings.TrimSpace(input.Title) == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_values", "Invalid values")
		return
	}

	newBook := Book{ID: nextID(books), Title: input.Title, Author: input.Author}

	books = append(books, newBook)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	res := SuccessResponse{
		Data: newBook,
	}
	json.NewEncoder(w).Encode(res)
}

func nextID(books []Book) int64 {
	var maxID int64 = 0
	for _, v := range books {
		if v.ID > maxID {
			maxID = v.ID
		}
	}
	return maxID + 1
}

func newMux(store BookStore) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /welcome", welcomeHandler)
	mux.HandleFunc("GET /books", bookHandler)
	mux.HandleFunc("GET /books/{id}", getBookByIDHandler(store))
	mux.HandleFunc("POST /books", createBookHandler)
	mux.HandleFunc("PATCH /books/{id}", updateBookHandler)
	mux.HandleFunc("DELETE /books/{id}", deleteBookHandler)
	return mux
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pingErr := db.PingContext(pingCtx)
	cancel()

	if pingErr != nil {
		log.Fatal(pingErr)
	}

	store := NewPostgresBookStore(db)
	mux := newMux(store)

	fmt.Println("Listening on port 8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
