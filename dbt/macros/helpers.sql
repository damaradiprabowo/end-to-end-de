{# ============================================================= #}
{# Custom macros for the minimarket project (ClickHouse).        #}
{# ============================================================= #}

{# Generate a tenant-scoped surrogate key from one or more parts.
   Natural ids collide across tenants, so every key is namespaced. #}
{% macro gen_sk(parts) %}
    concat(
        {%- for p in parts -%}
            toString({{ p }}){% if not loop.last %}, '-', {% endif %}
        {%- endfor -%}
    )
{% endmacro %}

{# Standardize a date-key (YYYYMMDD integer) from a datetime/date column. #}
{% macro date_key(col) %}
    toInt32(formatDateTime({{ col }}, '%Y%m%d'))
{% endmacro %}

{# Audit columns appended to every model for lineage/freshness. #}
{% macro audit_columns() %}
    now() AS dbt_loaded_at
{% endmacro %}
