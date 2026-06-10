"""
Intermediate pipeline DAG
=========================
Multi-tenant, incremental ELT orchestrated end-to-end:

    go_elt (Golang goroutines)  ->  dbt run (all)  ->  dbt test  ->  dbt docs

The Go binary loads all configured tenants concurrently into ClickHouse `raw`
using an incremental high-watermark, then dbt builds the full star schema
(adds dim_store, dim_promotion, fact_promotion_usage).
"""
from datetime import datetime, timedelta

from airflow import DAG
from airflow.operators.bash import BashOperator

DBT = "dbt --no-use-colors"
DBT_DIRS = "--project-dir $DBT_PROJECT_DIR --profiles-dir $DBT_PROFILES_DIR"

default_args = {
    "owner": "data-engineering",
    "retries": 1,
    "retry_delay": timedelta(minutes=1),
}

with DAG(
    dag_id="intermediate_pipeline",
    description="Intermediate: Go multi-tenant incremental EL -> dbt -> tests -> docs",
    schedule_interval=None,
    start_date=datetime(2024, 1, 1),
    catchup=False,
    default_args=default_args,
    tags=["intermediate", "elt", "golang", "dbt"],
) as dag:

    go_elt = BashOperator(
        task_id="go_extract_load",
        bash_command=(
            "mkdir -p /opt/airflow/state && "
            "elt --config /opt/airflow/pipeline/golang/config/tenants.json "
            "--watermark /opt/airflow/state/watermark.json"
        ),
    )

    dbt_run = BashOperator(
        task_id="dbt_run",
        bash_command=f"{DBT} run {DBT_DIRS}",
    )

    dbt_test = BashOperator(
        task_id="dbt_test",
        bash_command=f"{DBT} test {DBT_DIRS}",
    )

    dbt_docs = BashOperator(
        task_id="dbt_docs_generate",
        bash_command=f"{DBT} docs generate {DBT_DIRS}",
    )

    go_elt >> dbt_run >> dbt_test >> dbt_docs
