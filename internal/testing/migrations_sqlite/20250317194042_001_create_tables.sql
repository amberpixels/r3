-- +goose Up
CREATE TABLE cities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    country_name TEXT,
    popularity INTEGER
);

CREATE TABLE locations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    slug TEXT,
    city_id INTEGER REFERENCES cities(id) ON UPDATE CASCADE ON DELETE SET NULL,
    popularity INTEGER,
    visible INTEGER
);

CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    happened_at DATETIME,
    weight INTEGER,
    venue_id INTEGER REFERENCES locations(id) ON UPDATE CASCADE ON DELETE SET NULL,
    active INTEGER
);

CREATE TABLE artists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT
);

CREATE TABLE artist_to_events (
    artist_id INTEGER NOT NULL REFERENCES artists(id),
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    PRIMARY KEY (artist_id, event_id)
);

CREATE UNIQUE INDEX artist_to_events_artist_id_event_id_uniq ON artist_to_events (artist_id, event_id);

CREATE TABLE city_translations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    city_id INTEGER REFERENCES cities(id) ON UPDATE CASCADE ON DELETE CASCADE,
    locale TEXT
);

CREATE TABLE location_translations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    slug TEXT,
    location_id INTEGER REFERENCES locations(id) ON UPDATE CASCADE ON DELETE CASCADE,
    locale TEXT
);

CREATE TABLE event_translations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    event_id INTEGER REFERENCES events(id) ON UPDATE CASCADE ON DELETE CASCADE,
    locale TEXT
);

CREATE TABLE artist_translations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    artist_id INTEGER REFERENCES artists(id) ON UPDATE CASCADE ON DELETE CASCADE,
    locale TEXT
);

-- +goose Down
DROP TABLE IF EXISTS artist_translations;
DROP TABLE IF EXISTS event_translations;
DROP TABLE IF EXISTS location_translations;
DROP TABLE IF EXISTS city_translations;
DROP TABLE IF EXISTS artist_to_events;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS artists;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS cities;
