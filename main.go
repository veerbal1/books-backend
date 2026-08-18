package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
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

func bookHandler(store BookStore) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		listResult, err := store.ListBooks(r.Context(), ListOptions{
			Author:    author,
			Title:     title,
			SortField: sortField,
			Order:     order,
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			slog.Error("list books failed", "error", err)
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"internal_error",
				"Internal server error",
			)
			return
		}

		books := listResult.Books
		if books == nil {
			books = []Book{}
		}

		hasMore := offset+len(books) < listResult.Total

		res := ListResponse{
			Data: books,
			Pagination: Pagination{
				Limit:   limit,
				Offset:  offset,
				Total:   listResult.Total,
				HasMore: hasMore,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(res)
	})
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
			slog.Error("get book failed", "book_id", idInt, "error", err)
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

func updateBookHandler(store BookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		updatedBook, err := store.UpdateBook(r.Context(), idInt, input.Title, input.Author)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "book_not_found", "Request book not found")
			return
		}
		if err != nil {
			slog.Error("update book failed", "book_id", idInt, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		res := SuccessResponse{Data: updatedBook}
		json.NewEncoder(w).Encode(res)
	}
}

func deleteBookHandler(store BookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt, err := strconv.ParseInt(id, 10, 64)
		if err != nil || idInt <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
			return
		}

		err = store.DeleteBook(r.Context(), idInt)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "book_not_found", "Request book not found")
			return
		}
		if err != nil {
			slog.Error("delete book failed", "book_id", idInt, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func isJSONContentType(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	return mediaType == "application/json"
}

func createBookHandler(store BookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		newBook, err := store.CreateBook(r.Context(), input.Title, input.Author)
		if err != nil {
			slog.Error("create book failed", "error", err)
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"internal_error",
				"Internal server error",
			)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		res := SuccessResponse{
			Data: newBook,
		}
		json.NewEncoder(w).Encode(res)
	}
}

func newMux(store BookStore) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /welcome", welcomeHandler)
	mux.HandleFunc("GET /books", bookHandler(store))
	mux.HandleFunc("GET /books/{id}", getBookByIDHandler(store))
	mux.HandleFunc("POST /books", createBookHandler(store))
	mux.HandleFunc("PATCH /books/{id}", updateBookHandler(store))
	mux.HandleFunc("DELETE /books/{id}", deleteBookHandler(store))
	return mux
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rec *statusRecorder) WriteHeader(status int) {
	if rec.wroteHeader {
		return
	}

	rec.status = status
	rec.wroteHeader = true
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *statusRecorder) Write(body []byte) (int, error) {
	if !rec.wroteHeader {
		rec.WriteHeader(http.StatusOK)
	}

	return rec.ResponseWriter.Write(body)
}

func requestLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			attributes := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(started).Milliseconds(),
			}

			switch {
			case recorder.status >= http.StatusInternalServerError:
				logger.Error("request completed", attributes...)
			case recorder.status >= http.StatusBadRequest:
				logger.Warn("request completed", attributes...)
			default:
				logger.Info("request completed", attributes...)
			}
		})
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pingErr := db.PingContext(pingCtx)
	cancel()

	if pingErr != nil {
		return fmt.Errorf("failed to ping database: %w", pingErr)
	}

	store := NewPostgresBookStore(db)
	mux := newMux(store)
	requestLogger := requestLoggingMiddleware(logger)
	handler := requestLogger(mux)

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
	}
	logger.Info("server starting", "port", cfg.Port)
	err = server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
