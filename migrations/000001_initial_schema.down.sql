-- Rollback initial schema migration
-- Drops all tables created in 000001_initial_schema.up.sql

DROP TABLE IF EXISTS alarm_suggestions;
DROP TABLE IF EXISTS anomalies;
DROP TABLE IF EXISTS metrics;
