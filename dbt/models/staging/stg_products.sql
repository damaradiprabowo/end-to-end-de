with source as (
    select * from {{ source('raw', 'products') }} final
)
select
    {{ gen_sk(['tenant_id', 'product_id']) }} as product_sk,
    tenant_id,
    product_id,
    trim(product_name)               as product_name,
    category,
    brand,
    unit_price,
    is_active,
    created_at,
    updated_at
from source
