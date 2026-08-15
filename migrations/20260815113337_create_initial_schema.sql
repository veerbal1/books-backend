-- +goose Up
CREATE TABLE books (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL
);

CREATE TABLE authors (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE book_authors (
    book_id BIGINT NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES authors (id) ON DELETE RESTRICT,
    position INTEGER NOT NULL,
    PRIMARY KEY (book_id, author_id)
);

ALTER TABLE books
ADD CONSTRAINT books_title_not_blank CHECK (length(trim(title)) > 0);

ALTER TABLE authors
ADD CONSTRAINT authors_name_not_blank CHECK (length(trim(name)) > 0);

ALTER TABLE book_authors
ADD CONSTRAINT book_authors_position_positive CHECK (position > 0);

ALTER TABLE book_authors
ADD CONSTRAINT book_authors_book_position_unique UNIQUE (book_id, position);

CREATE INDEX book_authors_author_id_idx ON book_authors (author_id);

-- +goose Down
DROP TABLE book_authors;

DROP TABLE authors;

DROP TABLE books;