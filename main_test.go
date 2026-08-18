package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeBookStore struct {
	book       Book
	listResult ListResult
	listCall   *listCall
	updateCall *updateCall
	deleteCall *deleteCall
	err        error
}

type listCall struct {
	called  bool
	options ListOptions
}

type updateCall struct {
	called bool
	id     int64
	title  *string
	author *string
}

type deleteCall struct {
	called bool
	id     int64
}

func (f fakeBookStore) GetBookByID(
	ctx context.Context,
	id int64,
) (Book, error) {
	return f.book, f.err
}

func (f fakeBookStore) ListBooks(ctx context.Context, options ListOptions) (ListResult, error) {
	if f.listCall != nil {
		f.listCall.called = true
		f.listCall.options = options
	}
	return f.listResult, f.err
}

func (f fakeBookStore) CreateBook(
	ctx context.Context,
	title, author string,
) (Book, error) {
	return f.book, f.err
}

func (f fakeBookStore) UpdateBook(
	ctx context.Context,
	id int64,
	title, author *string,
) (Book, error) {
	if f.updateCall != nil {
		f.updateCall.called = true
		f.updateCall.id = id
		f.updateCall.title = cloneString(title)
		f.updateCall.author = cloneString(author)
	}
	return f.book, f.err
}

func (f fakeBookStore) DeleteBook(ctx context.Context, id int64) error {
	if f.deleteCall != nil {
		f.deleteCall.called = true
		f.deleteCall.id = id
	}
	return f.err
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func resetBooks(t *testing.T) {
	t.Helper()

	books = seedBooks()

	t.Cleanup(func() {
		books = seedBooks()
	})
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

	mux := newMux(fakeBookStore{})
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

	mux := newMux(fakeBookStore{})
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

	mux := newMux(fakeBookStore{err: sql.ErrNoRows})
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

func TestListBooksHandlerPassesOptionsAndReturnsStoreResult(t *testing.T) {
	call := &listCall{}
	mux := newMux(fakeBookStore{
		listCall: call,
		listResult: ListResult{
			Books: []Book{
				{ID: 3, Title: "Harry Potter", Author: "American"},
				{ID: 5, Title: "The Hobbit", Author: "American"},
			},
			Total: 5,
		},
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/books?author=American&title=Potter&sort=title&order=desc&limit=2&offset=1",
		nil,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
	}
	if !call.called {
		t.Fatal("ListBooks was not called")
	}

	wantOptions := ListOptions{
		Author:    "American",
		Title:     "Potter",
		SortField: "title",
		Order:     "desc",
		Limit:     2,
		Offset:    1,
	}
	if call.options != wantOptions {
		t.Fatalf("options = %#v, want %#v", call.options, wantOptions)
	}

	var body ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Data) != 2 || body.Data[0].ID != 3 || body.Data[1].ID != 5 {
		t.Fatalf("books = %#v, want IDs [3 5]", body.Data)
	}
	if body.Pagination != (Pagination{Limit: 2, Offset: 1, Total: 5, HasMore: true}) {
		t.Fatalf("pagination = %#v, want %#v", body.Pagination, Pagination{Limit: 2, Offset: 1, Total: 5, HasMore: true})
	}
}

func TestListBooksOrderBy(t *testing.T) {
	cases := []struct {
		name      string
		sortField string
		order     string
		want      string
		wantErr   bool
	}{
		{name: "ID ascending", sortField: "id", order: "asc", want: "b.id ASC"},
		{name: "title descending", sortField: "title", order: "desc", want: "LOWER(b.title) DESC, b.id ASC"},
		{name: "author ascending", sortField: "author", order: "asc", want: "LOWER(a.name) ASC, b.id ASC"},
		{name: "invalid field", sortField: "id; DROP TABLE books", order: "asc", wantErr: true},
		{name: "invalid order", sortField: "id", order: "sideways", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := listBooksOrderBy(tc.sortField, tc.order)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("listBooksOrderBy() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("order clause = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUpdateBookHandlerPassesPartialInput(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantTitle     string
		wantHasTitle  bool
		wantAuthor    string
		wantHasAuthor bool
		returnedBook  Book
	}{
		{
			name:         "title only",
			body:         `{"title":"New Title"}`,
			wantTitle:    "New Title",
			wantHasTitle: true,
			returnedBook: Book{ID: 1, Title: "New Title", Author: "Original Author"},
		},
		{
			name:          "author only",
			body:          `{"author":"New Author"}`,
			wantAuthor:    "New Author",
			wantHasAuthor: true,
			returnedBook:  Book{ID: 1, Title: "Original Title", Author: "New Author"},
		},
		{
			name:          "title and author",
			body:          `{"title":"New Title","author":"New Author"}`,
			wantTitle:     "New Title",
			wantHasTitle:  true,
			wantAuthor:    "New Author",
			wantHasAuthor: true,
			returnedBook:  Book{ID: 1, Title: "New Title", Author: "New Author"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := &updateCall{}
			mux := newMux(fakeBookStore{book: tc.returnedBook, updateCall: call})
			req := httptest.NewRequest(http.MethodPatch, "/books/1", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if !call.called || call.id != 1 {
				t.Fatalf("UpdateBook call = %#v, want ID 1", call)
			}
			if (call.title != nil) != tc.wantHasTitle {
				t.Fatalf("title supplied = %t, want %t", call.title != nil, tc.wantHasTitle)
			}
			if call.title != nil && *call.title != tc.wantTitle {
				t.Fatalf("title = %q, want %q", *call.title, tc.wantTitle)
			}
			if (call.author != nil) != tc.wantHasAuthor {
				t.Fatalf("author supplied = %t, want %t", call.author != nil, tc.wantHasAuthor)
			}
			if call.author != nil && *call.author != tc.wantAuthor {
				t.Fatalf("author = %q, want %q", *call.author, tc.wantAuthor)
			}

			var response struct {
				Data Book `json:"data"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response.Data != tc.returnedBook {
				t.Fatalf("response book = %#v, want %#v", response.Data, tc.returnedBook)
			}
		})
	}
}

func TestUpdateBookHandlerMissingBook(t *testing.T) {
	mux := newMux(fakeBookStore{err: sql.ErrNoRows})
	req := httptest.NewRequest(http.MethodPatch, "/books/999", strings.NewReader(`{"title":"New Title"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "book_not_found" {
		t.Fatalf("error code = %q, want %q", body.Error.Code, "book_not_found")
	}
}

func TestDeleteBookHandler(t *testing.T) {
	t.Run("deletes a book", func(t *testing.T) {
		call := &deleteCall{}
		mux := newMux(fakeBookStore{deleteCall: call})
		req := httptest.NewRequest(http.MethodDelete, "/books/7", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("body = %q, want empty", rec.Body.String())
		}
		if !call.called || call.id != 7 {
			t.Fatalf("DeleteBook call = %#v, want ID 7", call)
		}
	})

	t.Run("returns not found", func(t *testing.T) {
		mux := newMux(fakeBookStore{err: sql.ErrNoRows})
		req := httptest.NewRequest(http.MethodDelete, "/books/999", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}

		var body ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body.Error.Code != "book_not_found" {
			t.Fatalf("error code = %q, want %q", body.Error.Code, "book_not_found")
		}
	})
}
