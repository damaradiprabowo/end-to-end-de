# =====================================================================
# Advanced "transform" image, run by the advanced DAG via DockerOperator.
# Bundles:
#   * dbt-core + dbt-clickhouse (the staging/marts transformation + tests)
#   * the Python raw-layer DQ script (dq_raw.py) and its connectors, so the
#     same image serves the raw reconciliation gate as well.
# Build context = repository root (see docker-compose.yml `dbt` build service).
# =====================================================================
FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    DBT_PROFILES_DIR=/dbt \
    DBT_PROJECT_DIR=/dbt \
    PYTHONPATH=/pipeline/python

RUN pip install --no-cache-dir \
        dbt-core==1.7.13 \
        dbt-clickhouse==1.7.4 \
        clickhouse-connect==0.7.16 \
        psycopg2-binary==2.9.9

# dbt project (models, tests, macros, profiles) + the raw-DQ python script
COPY dbt /dbt
COPY pipeline/python /pipeline/python

WORKDIR /dbt
# Default command is overridden per-task by the DockerOperator.
CMD ["dbt", "--version"]
