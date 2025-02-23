BEGIN;

DROP INDEX IF EXISTS payments_user_id_item_id_idx;
DROP TABLE IF EXISTS payments;
DROP INDEX IF EXISTS transfers_src_user_id_idx;
DROP INDEX IF EXISTS transfers_dst_user_id_idx;
DROP TABLE IF EXISTS transfers;
DROP INDEX IF EXISTS items_name_idx;
DROP TABLE IF EXISTS items;
DROP INDEX IF EXISTS users_username_idx;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS "uuid-ossp";

COMMIT;
