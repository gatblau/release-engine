-- Create gitea database for Gitea
CREATE DATABASE gitea;
GRANT ALL PRIVILEGES ON DATABASE gitea TO release_engine;

-- Create additional schemas if needed
CREATE SCHEMA IF NOT EXISTS gitea_schema;
GRANT ALL ON SCHEMA gitea_schema TO release_engine;