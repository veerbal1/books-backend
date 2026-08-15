INSERT INTO
    authors (name)
VALUES ('George Orwell'),
    ('Jane Austen'),
    ('Neil Gaiman'),
    ('Terry Pratchett');

INSERT INTO
    books (title)
VALUES ('1984'),
    ('Pride and Prejudice'),
    ('Good Omens');

INSERT INTO
    book_authors (book_id, author_id, position)
VALUES (1, 1, 1),
    (2, 2, 1),
    (3, 4, 1),
    (3, 3, 2);