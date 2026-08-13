package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Book struct {
	ID     int    `json:"id"`
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

var books []Book = []Book{
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

func bookHandler(w http.ResponseWriter, r *http.Request) {
	author := r.URL.Query().Get("author")
	r.URL.Query().Get("limit")
	r.URL.Query().Get("offset")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if author != "" {
		tempBooks := []Book{}
		for _, book := range books {
			if author == book.Author {
				tempBooks = append(tempBooks, book)
			}
		}
		res := SuccessResponse{
			Data: tempBooks,
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	res := SuccessResponse{
		Data: books,
	}
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

func getBookByIDHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}
	if idInt <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}
	for _, book := range books {
		if book.ID == idInt {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			res := SuccessResponse{
				Data: book,
			}
			json.NewEncoder(w).Encode(res)
			return
		}
	}
	writeJSONError(w, http.StatusNotFound, "book_not_found", "Request book not found")
}

func updateBookHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idInt, err := strconv.Atoi(id)
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
	idInt, err := strconv.Atoi(id)
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

func createBookHandler(w http.ResponseWriter, r *http.Request) {
	var input CreateBookInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err != nil {
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

func nextID(books []Book) int {
	maxID := 0
	for _, v := range books {
		if v.ID > maxID {
			maxID = v.ID
		}
	}
	return maxID + 1
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /welcome", welcomeHandler)
	mux.HandleFunc("GET /books", bookHandler)
	mux.HandleFunc("GET /books/{id}", getBookByIDHandler)
	mux.HandleFunc("POST /books", createBookHandler)
	mux.HandleFunc("PATCH /books/{id}", updateBookHandler)
	mux.HandleFunc("DELETE /books/{id}", deleteBookHandler)

	fmt.Println("Listening on port 8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
