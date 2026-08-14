package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func resetBooks(t *testing.T) {
	t.Helper()

	books = seedBooks()

	t.Cleanup(func() {
		books = seedBooks()
	})
}

func TestNextID(t *testing.T) {
	books := []Book{
		{ID: 1},
		{ID: 2},
		{ID: 7},
	}
	cases := []struct {
		name  string
		books []Book
		want  int
	}{{
		name:  "highest ID plus one",
		books: books,
		want:  8,
	}, {
		name:  "no books starts at one",
		books: []Book{},
		want:  1,
	}, {
		name: "random books IDs",
		books: []Book{
			{ID: 4},
			{ID: 1},
			{ID: 3}},
		want: 5,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextID(tc.books)
			if got != tc.want {
				t.Fatalf("%s: nextID() = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestGetBookInvalidID(t *testing.T) {
	resetBooks(t)

	cases := []struct {
		name string
		path string
	}{
		{
			name: "non-numeric ID",
			path: "/books/abc",
		},
		{
			name: "zero ID",
			path: "/books/0",
		},
	}

	mux := newMux()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}

			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
			}

			var body ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if body.Error.Code != "invalid_id" {
				t.Fatalf("error code = %q, want %q", body.Error.Code, "invalid_id")
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux := newMux()
	mux.ServeHTTP(rec, req)

	code := rec.Code
	wantCode := http.StatusOK
	if code != wantCode {
		t.Fatalf("got = %d, want %d", code, wantCode)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("got = %s, want %s", contentType, "application/json")
	}

	var response struct {
		Data map[string]string `json:"data"`
	}
	err := json.NewDecoder(rec.Body).Decode(&response)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data["status"] != "ok" {
		t.Fatalf("status = %q, want %q", response.Data["status"], "ok")
	}
}

func TestGetMissingBook(t *testing.T) {
	resetBooks(t)

	req := httptest.NewRequest(http.MethodGet, "/books/999", nil)
	rec := httptest.NewRecorder()

	mux := newMux()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("got = %s, want %s", rec.Header().Get("Content-Type"), "application/json")
	}

	var body ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)

	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "book_not_found" {
		t.Fatalf("got = %s, want %s", body.Error.Code, "book_not_found")
	}
}
