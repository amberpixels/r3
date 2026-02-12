-- +goose Up
CREATE TABLE cities (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT,
    country_name TEXT,
    popularity BIGINT
);

CREATE TABLE locations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT,
    slug TEXT,
    city_id BIGINT,
    popularity BIGINT,
    visible BOOLEAN,
    FOREIGN KEY (city_id) REFERENCES cities(id) ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE TABLE events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT,
    happened_at DATETIME,
    weight BIGINT,
    venue_id BIGINT,
    active BOOLEAN,
    FOREIGN KEY (venue_id) REFERENCES locations(id) ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE TABLE artists (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT
);

CREATE TABLE artist_to_events (
    artist_id BIGINT NOT NULL,
    event_id BIGINT NOT NULL,
    PRIMARY KEY (artist_id, event_id),
    FOREIGN KEY (artist_id) REFERENCES artists(id),
    FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE
);

CREATE TABLE city_translations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT,
    city_id BIGINT,
    locale TEXT,
    FOREIGN KEY (city_id) REFERENCES cities(id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE location_translations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT,
    slug TEXT,
    location_id BIGINT,
    locale TEXT,
    FOREIGN KEY (location_id) REFERENCES locations(id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE event_translations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT,
    event_id BIGINT,
    locale TEXT,
    FOREIGN KEY (event_id) REFERENCES events(id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE artist_translations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name TEXT,
    artist_id BIGINT,
    locale TEXT,
    FOREIGN KEY (artist_id) REFERENCES artists(id) ON UPDATE CASCADE ON DELETE CASCADE
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
