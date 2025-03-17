-- +goose Up
-- +goose StatementBegin
create table public.cities (
    id bigserial primary key,
    name text,
    country_name text,
    popularity bigint
);

alter table public.cities owner to test;

create table public.locations (
    id bigserial primary key,
    name text,
    slug text,
    city_id bigint constraint fk_locations_city references public.cities on update cascade on delete set null,
    popularity bigint,
    visible boolean
);

alter table public.locations owner to test;

create table public.events (
    id bigserial primary key,
    happened_at timestamp
    with
        time zone,
        weight bigint,
        venue_id bigint constraint fk_events_location references public.locations on update cascade on delete set null,
        active boolean
);

alter table public.events owner to test;

create table public.artists (id bigserial primary key, name text);

alter table public.artists owner to test;

create table public.artist_to_events (
    artist_id integer not null references public.artists,
    event_id integer not null references public.events on delete cascade,
    primary key (artist_id, event_id)
);

create unique index artist_to_events_artist_id_event_id_uniq on public.artist_to_events (artist_id, event_id);

alter table public.artist_to_events owner to test;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS artists_to_events;

DROP TABLE IF EXISTS events;

DROP TABLE IF EXISTS artists;

DROP TABLE IF EXISTS locations;

DROP TABLE IF EXISTS cities;

-- +goose StatementEnd
