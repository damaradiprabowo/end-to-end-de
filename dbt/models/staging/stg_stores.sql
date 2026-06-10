with source as (
    select * from {{ source('raw', 'stores') }} final
)
select
    {{ gen_sk(['tenant_id', 'store_id']) }} as store_sk,
    tenant_id,
    store_id,
    trim(store_name)                 as store_name,
    city,
    province,
    store_type,
    opened_at,
    is_active,
    created_at,
    updated_at
from source
