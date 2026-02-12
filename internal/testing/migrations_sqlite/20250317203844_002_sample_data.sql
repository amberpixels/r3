-- +goose Up
-- +goose StatementBegin
INSERT INTO cities (name, country_name, popularity)
VALUES
    ('City One', 'Country A', 50),
    ('City Two', 'Country B', 70);

INSERT INTO city_translations (name, city_id, locale)
VALUES
    ('City One', 1, 'en'),
    ('Ciudad Uno', 1, 'es'),
    ('Stadt Eins', 1, 'de'),
    ('City Two', 2, 'en'),
    ('Ciudad Dos', 2, 'es'),
    ('Stadt Zwei', 2, 'de');

INSERT INTO locations (name, slug, city_id, popularity, visible)
VALUES
    ('Location One', 'loc-one', 1, 10, 1),
    ('Location Two', 'loc-two', 1, 20, 1),
    ('Location Three', 'loc-three', 1, 30, 0),
    ('Location Four', 'loc-four', 2, 40, 1),
    ('Location Five', 'loc-five', 2, 50, 1);

INSERT INTO location_translations (name, slug, location_id, locale)
VALUES
    ('Location One', 'loc-one', 1, 'en'),
    ('Lugar Uno', 'loc-one', 1, 'es'),
    ('Ort Eins', 'loc-one', 1, 'de'),
    ('Location Two', 'loc-two', 2, 'en'),
    ('Lugar Dos', 'loc-two', 2, 'es'),
    ('Ort Zwei', 'loc-two', 2, 'de'),
    ('Location Three', 'loc-three', 3, 'en'),
    ('Lugar Tres', 'loc-three', 3, 'es'),
    ('Ort Drei', 'loc-three', 3, 'de'),
    ('Location Four', 'loc-four', 4, 'en'),
    ('Lugar Cuatro', 'loc-four', 4, 'es'),
    ('Ort Vier', 'loc-four', 4, 'de'),
    ('Location Five', 'loc-five', 5, 'en'),
    ('Lugar Cinco', 'loc-five', 5, 'es'),
    ('Ort Fünf', 'loc-five', 5, 'de');

INSERT INTO events (happened_at, weight, venue_id, active)
VALUES
    (datetime('now', '+1 hour'), 101, 1, 1),
    (datetime('now', '+2 hours'), 102, 2, 1),
    (datetime('now', '+3 hours'), 103, 3, 1),
    (datetime('now', '+4 hours'), 104, 4, 1),
    (datetime('now', '+5 hours'), 105, 5, 1),
    (datetime('now', '+6 hours'), 106, 1, 1),
    (datetime('now', '+7 hours'), 107, 2, 1),
    (datetime('now', '+8 hours'), 108, 3, 1);

INSERT INTO event_translations (name, event_id, locale)
VALUES
    ('Event One', 1, 'en'),
    ('Evento Uno', 1, 'es'),
    ('Ereignis Eins', 1, 'de'),
    ('Event Two', 2, 'en'),
    ('Evento Dos', 2, 'es'),
    ('Ereignis Zwei', 2, 'de'),
    ('Event Three', 3, 'en'),
    ('Evento Tres', 3, 'es'),
    ('Ereignis Drei', 3, 'de'),
    ('Event Four', 4, 'en'),
    ('Evento Cuatro', 4, 'es'),
    ('Ereignis Vier', 4, 'de'),
    ('Event Five', 5, 'en'),
    ('Evento Cinco', 5, 'es'),
    ('Ereignis Fünf', 5, 'de'),
    ('Event Six', 6, 'en'),
    ('Evento Seis', 6, 'es'),
    ('Ereignis Sechs', 6, 'de'),
    ('Event Seven', 7, 'en'),
    ('Evento Siete', 7, 'es'),
    ('Ereignis Sieben', 7, 'de'),
    ('Event Eight', 8, 'en'),
    ('Evento Ocho', 8, 'es'),
    ('Ereignis Acht', 8, 'de');

INSERT INTO artists (name)
VALUES
    ('David Bowie'),
    ('Michael C. Hall'),
    ('Thom Yorke');

INSERT INTO artist_translations (name, artist_id, locale)
VALUES
    ('David Bowie', 1, 'en'),
    ('El David Bowie', 1, 'es'),
    ('Der David Bowie', 1, 'de'),
    ('Michael C. Hall', 2, 'en'),
    ('El Michael C. Hall', 2, 'es'),
    ('Der Michael C. Hall', 2, 'de'),
    ('Thom Yorke', 3, 'en'),
    ('El Thom Yorke', 3, 'es'),
    ('Der Thom Yorke', 3, 'de');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM artist_translations;
DELETE FROM event_translations;
DELETE FROM location_translations;
DELETE FROM city_translations;
DELETE FROM artist_to_events;
DELETE FROM events;
DELETE FROM artists;
DELETE FROM locations;
DELETE FROM cities;
-- +goose StatementEnd
