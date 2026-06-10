with customers as (
    select * from {{ ref('stg_customers') }}
)
select
    customer_sk,
    tenant_id,
    customer_id,
    customer_name,
    phone,
    email,
    gender,
    city,
    created_at as customer_since,
    {{ audit_columns() }}
from customers
