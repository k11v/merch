BEGIN;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    username text NOT NULL,
    password_hash text NOT NULL,
    balance integer NOT NULL DEFAULT 0,
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS users_username_idx ON users (username);

CREATE TABLE IF NOT EXISTS transactions (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    from_user_id uuid,
    to_user_id uuid,
    amount integer NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (from_user_id) REFERENCES users (id),
    FOREIGN KEY (to_user_id) REFERENCES users (id)
);

CREATE TABLE IF NOT EXISTS items (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    name text NOT NULL,
    price integer NOT NULL,
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS items_name_idx ON items (name);

CREATE TABLE IF NOT EXISTS users_items (
    user_id uuid NOT NULL,
    item_id uuid NOT NULL,
    amount integer NOT NULL,
    PRIMARY KEY (user_id, item_id),
    FOREIGN KEY (user_id) REFERENCES users (id),
    FOREIGN KEY (item_id) REFERENCES items (id)
);

COMMIT;
