-- Runs once, on the postgres container's first boot (empty data volume),
-- via /docker-entrypoint-initdb.d. Also mounted by the Kubernetes setup
-- (infra/k8s), so both environments initialise identically.
--
-- One database + one login role per service — database-per-service on a
-- shared server. Each role owns only its own database and can't connect to
-- the others. Passwords are read from the container's environment with
-- psql's \getenv (Postgres 15+), so none are written in this file; an
-- unset variable fails the init loudly rather than creating an empty
-- password.
--
-- Tables are NOT created here: each service applies its own embedded
-- migrations on boot (Server/app/internal/services/<svc>/migrations).

\set ON_ERROR_STOP on

\getenv catalog_password CATALOG_DB_PASSWORD
\getenv borrow_password  BORROW_DB_PASSWORD
\getenv user_password    USER_DB_PASSWORD
\getenv payment_password PAYMENT_DB_PASSWORD

-- catalog-service
CREATE ROLE catalog LOGIN PASSWORD :'catalog_password';
CREATE DATABASE bibliomania_catalog OWNER catalog;
REVOKE CONNECT ON DATABASE bibliomania_catalog FROM PUBLIC;
GRANT CONNECT ON DATABASE bibliomania_catalog TO catalog;

-- borrow-service
CREATE ROLE borrow LOGIN PASSWORD :'borrow_password';
CREATE DATABASE bibliomania_borrow OWNER borrow;
REVOKE CONNECT ON DATABASE bibliomania_borrow FROM PUBLIC;
GRANT CONNECT ON DATABASE bibliomania_borrow TO borrow;

-- user-service ("user" is a reserved word in Postgres — the role is "users")
CREATE ROLE users LOGIN PASSWORD :'user_password';
CREATE DATABASE bibliomania_user OWNER users;
REVOKE CONNECT ON DATABASE bibliomania_user FROM PUBLIC;
GRANT CONNECT ON DATABASE bibliomania_user TO users;

-- payment-service
CREATE ROLE payment LOGIN PASSWORD :'payment_password';
CREATE DATABASE bibliomania_payment OWNER payment;
REVOKE CONNECT ON DATABASE bibliomania_payment FROM PUBLIC;
GRANT CONNECT ON DATABASE bibliomania_payment TO payment;
