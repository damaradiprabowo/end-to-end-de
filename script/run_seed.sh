#!/usr/bin/env bash
# =====================================================================
# run_seed.sh — populate the OLTP PostgreSQL with schema + fake data.
#
# Usage:
#   ./script/run_seed.sh
#
# It will:
#   1. wait for Postgres to be ready
#   2. apply the DDL (script/init/01_schema.sql)
#   3. run the Faker seed generator (script/seed/generate_seed.py)
#
# Reads connection settings from the environment (.env). When run from the
# host, defaults point to localhost; inside docker-compose they are injected.
# =====================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Load .env if present
if [[ -f "${ROOT_DIR}/.env" ]]; then
  set -a; source "${ROOT_DIR}/.env"; set +a
fi

export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export POSTGRES_DB="${POSTGRES_DB:-pos}"
export POSTGRES_USER="${POSTGRES_USER:-pos_user}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-pos_password}"
export PGPASSWORD="${POSTGRES_PASSWORD}"

echo ">> Waiting for PostgreSQL at ${POSTGRES_HOST}:${POSTGRES_PORT} ..."
for i in {1..30}; do
  if pg_isready -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" >/dev/null 2>&1; then
    echo ">> PostgreSQL is ready."
    break
  fi
  sleep 2
  if [[ $i -eq 30 ]]; then echo "!! Postgres not ready, aborting"; exit 1; fi
done

echo ">> Applying DDL ..."
psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" \
     -d "${POSTGRES_DB}" -v ON_ERROR_STOP=1 -f "${SCRIPT_DIR}/init/01_schema.sql"

echo ">> Installing seed dependencies ..."
python3 -m pip install -q -r "${SCRIPT_DIR}/seed/requirements.txt"

echo ">> Generating seed data ..."
python3 "${SCRIPT_DIR}/seed/generate_seed.py"

echo ">> Seed complete."
