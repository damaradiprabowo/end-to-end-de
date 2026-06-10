with source as (
    select * from {{ source('raw', 'suppliers') }} final
)
select
    {{ gen_sk(['tenant_id', 'supplier_id']) }} as supplier_sk,
    tenant_id,
    supplier_id,
    supplier_name,
    contact_name,
    city,
    country,
    created_at,
    updated_at
from source
