package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPostgresCRUDIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	resetDatabase := func() {
		if _, err := db.ExecContext(
			context.Background(),
			`TRUNCATE TABLE book_authors, books, authors RESTART IDENTITY`,
		); err != nil {
			t.Fatalf("reset test database: %v", err)
		}
	}
	resetDatabase()
	t.Cleanup(resetDatabase)

	store := NewPostgresBookStore(db)
	server := httptest.NewServer(newMux(store))
	t.Cleanup(server.Close)

	createRequest, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/books",
		strings.NewReader(`{"title":"Integration Book","author":"Integration Author"}`),
	)
	if err != nil {
		t.Fatalf("build create request: %v", err)
	}
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse, err := server.Client().Do(createRequest)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	defer createResponse.Body.Close()
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createResponse.StatusCode, http.StatusCreated)
	}

	var created struct {
		Data Book `json:"data"`
	}
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode created book: %v", err)
	}
	if created.Data.ID < 1 {
		t.Fatalf("created ID = %d, want a database-generated positive ID", created.Data.ID)
	}

	bookURL := fmt.Sprintf("%s/books/%d", server.URL, created.Data.ID)
	getResponse, err := server.Client().Get(bookURL)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	defer getResponse.Body.Close()
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getResponse.StatusCode, http.StatusOK)
	}

	listResponse, err := server.Client().Get(server.URL + "/books?author=Integration%20Author&title=Integration&sort=title&order=desc&limit=1")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer listResponse.Body.Close()
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResponse.StatusCode, http.StatusOK)
	}
	var listed ListResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Data) != 1 || listed.Data[0].ID != created.Data.ID || listed.Pagination.Total != 1 {
		t.Fatalf("list response = %#v, want one created book", listed)
	}

	patchRequest, err := http.NewRequest(
		http.MethodPatch,
		bookURL,
		strings.NewReader(`{"title":"Integration Book Updated","author":"Integration Author Updated"}`),
	)
	if err != nil {
		t.Fatalf("build patch request: %v", err)
	}
	patchRequest.Header.Set("Content-Type", "application/json")
	patchResponse, err := server.Client().Do(patchRequest)
	if err != nil {
		t.Fatalf("patch request: %v", err)
	}
	defer patchResponse.Body.Close()
	if patchResponse.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d, want %d", patchResponse.StatusCode, http.StatusOK)
	}
	var patched struct {
		Data Book `json:"data"`
	}
	if err := json.NewDecoder(patchResponse.Body).Decode(&patched); err != nil {
		t.Fatalf("decode patched book: %v", err)
	}
	if patched.Data != (Book{ID: created.Data.ID, Title: "Integration Book Updated", Author: "Integration Author Updated"}) {
		t.Fatalf("patched book = %#v", patched.Data)
	}

	if _, err := db.ExecContext(context.Background(), `
		CREATE FUNCTION integration_force_link_failure() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'forced relationship failure';
		END;
		$$`); err != nil {
		t.Fatalf("create forced-failure function: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
		CREATE TRIGGER integration_force_link_failure
		BEFORE INSERT ON book_authors
		FOR EACH ROW EXECUTE FUNCTION integration_force_link_failure()`); err != nil {
		t.Fatalf("create forced-failure trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(
			context.Background(),
			`DROP TRIGGER IF EXISTS integration_force_link_failure ON book_authors`,
		); err != nil {
			t.Errorf("remove forced-failure trigger: %v", err)
		}
		if _, err := db.ExecContext(
			context.Background(),
			`DROP FUNCTION IF EXISTS integration_force_link_failure()`,
		); err != nil {
			t.Errorf("remove forced-failure function: %v", err)
		}
	})

	rollbackTitle := "This title must roll back"
	rollbackAuthor := "This author must roll back"
	if _, err := store.UpdateBook(ctx, created.Data.ID, &rollbackTitle, &rollbackAuthor); err == nil {
		t.Fatal("expected update transaction to fail")
	}

	bookAfterFailedUpdate, err := store.GetBookByID(ctx, created.Data.ID)
	if err != nil {
		t.Fatalf("get book after failed update: %v", err)
	}
	if bookAfterFailedUpdate != patched.Data {
		t.Fatalf("book after failed update = %#v, want %#v", bookAfterFailedUpdate, patched.Data)
	}

	if _, err := store.CreateBook(ctx, "This book must roll back", "This author must roll back"); err == nil {
		t.Fatal("expected create transaction to fail")
	}
	var partialRows int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM books
		WHERE title = 'This book must roll back'`,
	).Scan(&partialRows); err != nil {
		t.Fatalf("count rolled-back books: %v", err)
	}
	if partialRows != 0 {
		t.Fatalf("rolled-back book rows = %d, want 0", partialRows)
	}
	var partialAuthors int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM authors WHERE name IN ($1, $2)`,
		rollbackAuthor,
		"This author must roll back",
	).Scan(&partialAuthors); err != nil {
		t.Fatalf("count rolled-back authors: %v", err)
	}
	if partialAuthors != 0 {
		t.Fatalf("rolled-back author rows = %d, want 0", partialAuthors)
	}

	deleteRequest, err := http.NewRequest(http.MethodDelete, bookURL, nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	deleteResponse, err := server.Client().Do(deleteRequest)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer deleteResponse.Body.Close()
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", deleteResponse.StatusCode, http.StatusNoContent)
	}

	missingResponse, err := server.Client().Get(bookURL)
	if err != nil {
		t.Fatalf("get deleted book: %v", err)
	}
	defer missingResponse.Body.Close()
	if missingResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d, want %d", missingResponse.StatusCode, http.StatusNotFound)
	}
}
