"""
Advanced pipeline DAG
=====================
Production-grade, multi-tenant ELT orchestrated with the **DockerOperator**, so
every stage runs in its own purpose-built, version-pinned container instead of
inside the Airflow image. A **fail-fast data-quality gate runs after each layer**
— a broken layer stops the pipeline before the next one is built:

    elt_advanced            (Go fan-out/fan-in loader, DWH watermark)
        |
    dq_raw                  RAW gate   — row counts reconciled vs PostgreSQL
        |
    dbt_run_staging
        |
    dq_staging              STAGING gate — source null checks + staging tests
        |
    dbt_run_marts           (intermediate + marts)
        |
    dq_marts                MARTS gate  — relationships, accepted_values, +
                                          singular DQ tests (revenue, freshness)
        |
    dbt_docs_generate

Images (built by `docker compose build elt dbt`):
  * minimarket/elt:latest  — Go ELT binaries (elt, elt-advanced)
  * minimarket/dbt:latest  — dbt-core + dbt-clickhouse + raw-DQ python script

The Airflow scheduler must have the Docker socket mounted and share the compose
network so the task containers can reach `postgres` / `clickhouse` by name
(see docker-compose.yml).
"""
import os
from datetime import datetime, timedelta

from airflow import DAG
from airflow.providers.docker.operators.docker import DockerOperator

# Compose network the task containers join so they can resolve postgres/clickhouse.
NETWORK = os.getenv("PIPELINE_DOCKER_NETWORK", "end-to-end-de-platform_default")
DOCKER_URL = os.getenv("DOCKER_URL", "unix://var/run/docker.sock")

ELT_IMAGE = "minimarket/elt:latest"
DBT_IMAGE = "minimarket/dbt:latest"

# Connection env shared by every task container (service names on the compose net).
CONN_ENV = {
    "POSTGRES_HOST": "postgres",
    "POSTGRES_PORT": "5432",
    "POSTGRES_DB": os.getenv("POSTGRES_DB", "pos"),
    "POSTGRES_USER": os.getenv("POSTGRES_USER", "pos_user"),
    "POSTGRES_PASSWORD": os.getenv("POSTGRES_PASSWORD", "pos_password"),
    "CLICKHOUSE_HOST": "clickhouse",
    "CLICKHOUSE_PORT": "9000",          # native protocol (Go loader)
    "CLICKHOUSE_HTTP_PORT": "8123",     # http protocol (dbt + dq_raw)
    "CLICKHOUSE_USER": os.getenv("CLICKHOUSE_USER", "default"),
    "CLICKHOUSE_PASSWORD": os.getenv("CLICKHOUSE_PASSWORD", "clickhouse"),
    "CLICKHOUSE_RAW_DB": os.getenv("CLICKHOUSE_RAW_DB", "raw"),
    "CLICKHOUSE_ANALYTICS_DB": os.getenv("CLICKHOUSE_ANALYTICS_DB", "analytics"),
    "TENANT_SCHEMAS": os.getenv("TENANT_SCHEMAS", "tenant_01,tenant_02,tenant_03"),
}

# kwargs common to every DockerOperator task
DOCKER_DEFAULTS = dict(
    api_version="auto",
    docker_url=DOCKER_URL,
    network_mode=NETWORK,
    auto_remove="force",
    mount_tmp_dir=False,
    environment=CONN_ENV,
)

DBT_FLAGS = "--no-use-colors --project-dir /dbt --profiles-dir /dbt"

default_args = {
    "owner": "data-engineering",
    "retries": 1,
    "retry_delay": timedelta(minutes=1),
}

with DAG(
    dag_id="advanced_pipeline",
    description="Advanced: DockerOperator ELT with a fail-fast DQ gate per layer",
    schedule_interval=None,
    start_date=datetime(2024, 1, 1),
    catchup=False,
    default_args=default_args,
    tags=["advanced", "elt", "golang", "dbt", "docker", "dq"],
) as dag:

    # ---- EL: Go fan-out/fan-in loader, incremental DWH-watermark ----
    elt_advanced = DockerOperator(
        task_id="elt_advanced",
        image=ELT_IMAGE,
        entrypoint="elt-advanced",
        command="--config config/tenants.json",
        **DOCKER_DEFAULTS,
    )

    # ---- RAW gate: row-count reconciliation against PostgreSQL ----
    dq_raw = DockerOperator(
        task_id="dq_raw",
        image=DBT_IMAGE,
        command="python /pipeline/python/dq_raw.py",
        **DOCKER_DEFAULTS,
    )

    # ---- STAGING build + gate ----
    dbt_run_staging = DockerOperator(
        task_id="dbt_run_staging",
        image=DBT_IMAGE,
        command=f"dbt run {DBT_FLAGS} --select path:models/staging",
        **DOCKER_DEFAULTS,
    )
    dq_staging = DockerOperator(
        task_id="dq_staging",
        image=DBT_IMAGE,
        # cautious indirect selection: only run a test when ALL of its parents
        # are in the selection, so singular tests that also depend on the (not
        # yet built) marts are not pulled into the staging gate.
        command=(f"dbt test {DBT_FLAGS} --select source:raw path:models/staging "
                 f"--indirect-selection=cautious"),
        **DOCKER_DEFAULTS,
    )

    # ---- MARTS build (intermediate + marts) + gate ----
    dbt_run_marts = DockerOperator(
        task_id="dbt_run_marts",
        image=DBT_IMAGE,
        command=f"dbt run {DBT_FLAGS} --select path:models/intermediate path:models/marts",
        **DOCKER_DEFAULTS,
    )
    dq_marts = DockerOperator(
        task_id="dq_marts",
        image=DBT_IMAGE,
        # full test run covers mart generic tests + the singular DQ tests
        # (revenue reconciliation + freshness/SLA) that depend on fact_sales.
        command=f"dbt test {DBT_FLAGS} --exclude path:models/staging",
        **DOCKER_DEFAULTS,
    )

    # ---- docs ----
    dbt_docs = DockerOperator(
        task_id="dbt_docs_generate",
        image=DBT_IMAGE,
        command=f"dbt docs generate {DBT_FLAGS}",
        **DOCKER_DEFAULTS,
    )

    elt_advanced >> dq_raw >> dbt_run_staging >> dq_staging \
        >> dbt_run_marts >> dq_marts >> dbt_docs
