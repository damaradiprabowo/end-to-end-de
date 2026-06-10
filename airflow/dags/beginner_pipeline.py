"""
Beginner pipeline DAG
=====================
Single-tenant, full-load ELT orchestrated end-to-end:

    extract_load (Python)  ->  dbt run (core models)  ->  dbt test

Loads tenant_01 from PostgreSQL into ClickHouse `raw`, then builds the
Beginner star schema (dim_customer, dim_product, dim_date, fact_sales).
"""
from datetime import datetime, timedelta

from airflow import DAG
from airflow.operators.bash import BashOperator

DBT = "dbt --no-use-colors"
DBT_DIRS = "--project-dir $DBT_PROJECT_DIR --profiles-dir $DBT_PROFILES_DIR"

# Beginner-scope models only
CORE_MODELS = (
    "stg_customers stg_products stg_transactions stg_transaction_items "
    "dim_customer dim_product dim_date fact_sales"
)

default_args = {
    "owner": "data-engineering",
    "retries": 1,
    "retry_delay": timedelta(minutes=1),
}

with DAG(
    dag_id="beginner_pipeline",
    description="Beginner: Python full-load EL -> dbt -> tests",
    schedule_interval=None,
    start_date=datetime(2024, 1, 1),
    catchup=False,
    default_args=default_args,
    tags=["beginner", "elt", "dbt"],
) as dag:

    extract_load = BashOperator(
        task_id="extract_load",
        bash_command=(
            "python /opt/airflow/pipeline/python/extract_load.py "
            "--tenant ${BEGINNER_TENANT:-tenant_01}"
        ),
    )

    dbt_run = BashOperator(
        task_id="dbt_run_core",
        bash_command=f"{DBT} run {DBT_DIRS} --select {CORE_MODELS}",
    )

    dbt_test = BashOperator(
        task_id="dbt_test_core",
        bash_command=f"{DBT} test {DBT_DIRS} --select {CORE_MODELS}",
    )

    extract_load >> dbt_run >> dbt_test
