with stores as (
    select * from {{ ref('stg_stores') }}
)
select
    store_sk,
    tenant_id,
    store_id,
    store_name,
    city,
    province,
    store_type,
    opened_at,
    is_active,
    {{ audit_columns() }}
from stores
