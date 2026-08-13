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
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if idInt <= 0 {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
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
	http.Error(w, "Request book not found", http.StatusNotFound)
}

func createBookHandler(w http.ResponseWriter, r *http.Request) {
	var input CreateBookInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(input.Author) == "" || strings.TrimSpace(input.Title) == "" {
		http.Error(w, "Invalid values", http.StatusBadRequest)
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

	fmt.Println("Listening on port 8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
