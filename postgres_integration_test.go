package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func resetTestDatabase(t *testing.T, db *sql.DB) {
	t.Helper()

	reset := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := db.ExecContext(
			ctx,
			`TRUNCATE TABLE book_authors, books, authors RESTART IDENTITY`,
		)
		if err != nil {
			t.Fatalf("reset test database: %v", err)
		}
	}

	reset()
	t.Cleanup(reset)
}

func openTestDatabase(t *testing.T) *sql.DB {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	resetTestDatabase(t, db)

	return db
}

func TestPostgresConnection(t *testing.T) {
	openTestDatabase(t)
}

func TestPostgresBookStoreCreateAndGet(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPostgresBookStore(db)

	ctx := context.Background()
	created, err := store.CreateBook(ctx, "Integration Book", "Integration Author")
	if err != nil {
		t.Fatalf("not expected err: %v", err)
		return
	}

	if created.ID <= 0 {
		t.Fatalf("created ID = %d, want a positive ID", created.ID)
	}

	if created.Title != "Integration Book" || created.Author != "Integration Author" {
		t.Fatalf(
			"created book = %#v, want title %q and author %q",
			created,
			"Integration Book",
			"Integration Author",
		)
	}

	got, err := store.GetBookByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get created book: %v", err)
	}

	if got != created {
		t.Fatalf("got book = %#v, want %#v", got, created)
	}
}
