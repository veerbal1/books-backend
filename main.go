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

type ErrorResponse struct {
	Error string `json:"error"`
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

func bookHandler(w http.ResponseWriter, r *http.Request) {
	author := r.URL.Query().Get("author")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if author != "" {
		tempBooks := []Book{}
		for _, book := range books {
			if author == book.Author {
				tempBooks = append(tempBooks, book)
			}
		}
		json.NewEncoder(w).Encode(tempBooks)
		return
	}
	json.NewEncoder(w).Encode(books)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Welcome to the jungle."})
}

func getBookByIDHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	if idInt <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	for _, book := range books {
		if book.ID == idInt {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(book)
			return
		}
	}
	writeJSONError(w, http.StatusNotFound, "Request book not found")
}

func updateBookHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	if idInt <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	for index, book := range books {
		if book.ID == idInt {
			var input UpdateBookInput
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&input)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "Invalid request")
				return
			}
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				writeJSONError(w, http.StatusBadRequest, "Invalid request")
				return
			}

			if input.Title != nil {
				trimmed := strings.TrimSpace(*input.Title)
				if trimmed == "" {
					writeJSONError(w, http.StatusBadRequest, "Invalid values")
					return
				}
				*input.Title = trimmed
			}
			if input.Author != nil {
				trimmed := strings.TrimSpace(*input.Author)
				if trimmed == "" {
					writeJSONError(w, http.StatusBadRequest, "Invalid values")
					return
				}
				*input.Author = trimmed
			}
			if input.Title == nil && input.Author == nil {
				writeJSONError(w, http.StatusBadRequest, "Invalid request")
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
			json.NewEncoder(w).Encode(books[index])
			return
		}
	}
	writeJSONError(w, http.StatusNotFound, "Request book not found")
}

func deleteBookHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	idInt, err := strconv.Atoi(id)
	if err != nil || idInt <= 0 {
		writeJSONError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	for index, book := range books {
		if book.ID == idInt {
			books = append(books[:index], books[index+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	writeJSONError(w, http.StatusNotFound, "Request book not found")
}

func createBookHandler(w http.ResponseWriter, r *http.Request) {
	var input CreateBookInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if strings.TrimSpace(input.Author) == "" || strings.TrimSpace(input.Title) == "" {
		writeJSONError(w, http.StatusBadRequest, "Invalid values")
		return
	}

	newBook := Book{ID: nextID(books), Title: input.Title, Author: input.Author}

	books = append(books, newBook)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newBook)
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
	http.HandleFunc("GET /health", healthHandler)
	http.HandleFunc("GET /welcome", welcomeHandler)
	http.HandleFunc("GET /books", bookHandler)
	http.HandleFunc("GET /books/{id}", getBookByIDHandler)
	http.HandleFunc("POST /books", createBookHandler)
	http.HandleFunc("PATCH /books/{id}", updateBookHandler)
	http.HandleFunc("DELETE /books/{id}", deleteBookHandler)

	fmt.Println("Listening on port 8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
