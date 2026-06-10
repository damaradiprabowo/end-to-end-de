-- Mart DQ: freshness / SLA check.
-- The fact must have been (re)built recently. dbt_loaded_at is stamped at run
-- time by the audit_columns() macro; if the most recent load is older than the
-- SLA window the data is considered stale and the test fails.
{% set sla_hours = 24 %}
select
    max(dbt_loaded_at) as last_loaded_at,
    now()              as checked_at
from {{ ref('fact_sales') }}
having dateDiff('hour', max(dbt_loaded_at), now()) > {{ sla_hours }}
