-- path: db/migrations/000001_init_schema.down.sql

DROP TABLE IF EXISTS logs CASCADE;
DROP TABLE IF EXISTS metrics CASCADE;
DROP TABLE IF EXISTS spans CASCADE;
