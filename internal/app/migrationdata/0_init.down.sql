BEGIN;

DROP TABLE IF EXISTS users_items;
DROP INDEX IF EXISTS items_name_idx;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS transfers;
DROP INDEX IF EXISTS users_username_idx;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS "uuid-ossp";

COMMIT;
