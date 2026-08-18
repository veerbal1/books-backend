package main

import (
	"context"
	"database/sql"
	"fmt"
)

type PostgresBookStore struct {
	db *sql.DB
}

func NewPostgresBookStore(db *sql.DB) *PostgresBookStore {
	return &PostgresBookStore{db: db}
}

type BookStore interface {
	GetBookByID(ctx context.Context, id int64) (Book, error)
	ListBooks(ctx context.Context, options ListOptions) (ListResult, error)
	CreateBook(ctx context.Context, title string, author string) (Book, error)
}

type ListOptions struct {
	Author    string
	Title     string
	SortField string
	Order     string
	Limit     int
	Offset    int
}

type ListResult struct {
	Books []Book
	Total int
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

func (s *PostgresBookStore) ListBooks(ctx context.Context, options ListOptions) (ListResult, error) {
	orderBy, err := listBooksOrderBy(options.SortField, options.Order)
	if err != nil {
		return ListResult{}, err
	}

	const fromAndWhere = `
		FROM books AS b
		JOIN book_authors AS ba ON ba.book_id = b.id
		JOIN authors AS a ON a.id = ba.author_id
		WHERE ba.position = 1
			AND ($1 = '' OR LOWER(a.name) = LOWER($1))
			AND ($2 = '' OR POSITION(LOWER($2) IN LOWER(b.title)) > 0)`

	countQuery := "SELECT COUNT(*) " + fromAndWhere
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, options.Author, options.Title).Scan(&total); err != nil {
		return ListResult{}, err
	}

	pageQuery := `SELECT b.id, b.title, a.name ` + fromAndWhere +
		` ORDER BY ` + orderBy + ` LIMIT $3 OFFSET $4`
	rows, err := s.db.QueryContext(
		ctx,
		pageQuery,
		options.Author,
		options.Title,
		options.Limit,
		options.Offset,
	)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	books := []Book{}
	for rows.Next() {
		var book Book
		if err := rows.Scan(&book.ID, &book.Title, &book.Author); err != nil {
			return ListResult{}, err
		}
		books = append(books, book)
	}

	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}

	return ListResult{Books: books, Total: total}, nil
}

func (s *PostgresBookStore) CreateBook(ctx context.Context, title string, author string) (Book, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback()

	var authorID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO authors (name) VALUES ($1) RETURNING id;`, author).Scan(&authorID); err != nil {
		return Book{}, err
	}

	var bookID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO books (title) VALUES ($1) RETURNING id;`, title).Scan(&bookID); err != nil {
		return Book{}, err
	}

	if _, err = tx.ExecContext(ctx, `INSERT INTO book_authors (book_id, author_id, position) VALUES ($1, $2, 1);`, bookID, authorID); err != nil {
		return Book{}, err
	}

	if err := tx.Commit(); err != nil {
		return Book{}, err
	}

	return Book{ID: bookID, Title: title, Author: author}, nil
}

func listBooksOrderBy(sortField, order string) (string, error) {
	columns := map[string]string{
		"id":     "b.id",
		"title":  "LOWER(b.title)",
		"author": "LOWER(a.name)",
	}

	column, ok := columns[sortField]
	if !ok {
		return "", fmt.Errorf("unsupported sort field: %q", sortField)
	}

	direction := map[string]string{
		"asc":  "ASC",
		"desc": "DESC",
	}[order]
	if direction == "" {
		return "", fmt.Errorf("unsupported sort order: %q", order)
	}

	if sortField == "id" {
		return column + " " + direction, nil
	}

	return column + " " + direction + ", b.id ASC", nil
}
