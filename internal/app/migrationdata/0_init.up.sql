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
CREATE INDEX IF NOT EXISTS transactions_from_user_id_idx ON transactions (from_user_id);
CREATE INDEX IF NOT EXISTS transactions_to_user_id_idx ON transactions (to_user_id);

CREATE TABLE IF NOT EXISTS items (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    name text NOT NULL,
    price integer NOT NULL,
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS items_name_idx ON items (name);
INSERT INTO items (id, name, price)
VALUES ('158acfe8-95cc-4573-b3e7-4edf89823f45', 't-shirt', 80),
       ('5317464e-36c7-4678-98af-2e2cf3138ec0', 'cup', 20),
       ('93e4b592-77c1-4928-8e45-b0ec86cdf035', 'book', 50),
       ('95d5a283-97be-4fb8-b3b4-0ef723877b7a', 'pen', 10),
       ('133ce041-eac7-4f99-81ce-f72e23ec7399', 'powerbank', 200),
       ('8f6e5480-a188-4626-a1de-87adbddbc932', 'hoody', 300),
       ('c4273781-c5ed-4fe3-8554-eb5ee4e1ba00', 'umbrella', 200),
       ('9b5da2b6-1024-48b4-81f7-e56ca9c711fe', 'socks', 10),
       ('f4c77264-a433-4775-ac80-1898517ff8cd', 'wallet', 50),
       ('4ca8f1f5-d719-42bf-8939-5e7ef51885c2', 'pink-hoody', 500)
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS users_items (
    user_id uuid NOT NULL,
    item_id uuid NOT NULL,
    amount integer NOT NULL,
    CONSTRAINT users_user_id_item_id_pkey PRIMARY KEY (user_id, item_id),
    FOREIGN KEY (user_id) REFERENCES users (id),
    FOREIGN KEY (item_id) REFERENCES items (id)
);

COMMIT;
