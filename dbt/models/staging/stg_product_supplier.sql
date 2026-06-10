with source as (
    select * from {{ source('raw', 'product_supplier') }} final
)
select
    {{ gen_sk(['tenant_id', 'id']) }}          as product_supplier_sk,
    tenant_id,
    id                                         as product_supplier_id,
    {{ gen_sk(['tenant_id', 'product_id']) }}  as product_sk,
    {{ gen_sk(['tenant_id', 'supplier_id']) }} as supplier_sk,
    product_id,
    supplier_id,
    cost_price,
    lead_time_days,
    is_primary,
    created_at,
    updated_at
from source
