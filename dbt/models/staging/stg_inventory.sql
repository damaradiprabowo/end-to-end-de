with source as (
    select * from {{ source('raw', 'inventory') }} final
)
select
    {{ gen_sk(['tenant_id', 'inventory_id']) }} as inventory_sk,
    tenant_id,
    inventory_id,
    {{ gen_sk(['tenant_id', 'product_id']) }}   as product_sk,
    {{ gen_sk(['tenant_id', 'store_id']) }}     as store_sk,
    {{ gen_sk(['tenant_id', 'supplier_id']) }}  as supplier_sk,
    product_id,
    store_id,
    supplier_id,
    stock_qty,
    reorder_level,
    last_restocked,
    created_at,
    updated_at
from source
