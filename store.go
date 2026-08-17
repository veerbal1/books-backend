package main

import (
	"context"
	"database/sql"
)

type PostgresBookStore struct {
	db *sql.DB
}

func NewPostgresBookStore(db *sql.DB) *PostgresBookStore {
	return &PostgresBookStore{db: db}
}

type BookStore interface {
	GetBookByID(ctx context.Context, id int64) (Book, error)
}

func (s *PostgresBookStore) GetBookByID(
	ctx context.Context,
	id int64,
) (Book, error) {
	query := `SELECT b.id, b.title, a.name
FROM
    books AS b
    JOIN book_authors AS ba ON ba.book_id = b.id
    JOIN authors AS a ON a.id = ba.author_id
WHERE
    b.id = $1
    AND ba.position = 1;`

	var book Book

	err := s.db.QueryRowContext(ctx, query, id).Scan(&book.ID, &book.Title, &book.Author)

	if err != nil {
		return Book{}, err
	}

	return book, nil
}
