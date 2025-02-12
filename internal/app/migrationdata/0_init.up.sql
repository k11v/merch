BEGIN;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    username text NOT NULL,
    password_hash text NOT NULL,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX users_username_idx ON users (username);

COMMIT;
