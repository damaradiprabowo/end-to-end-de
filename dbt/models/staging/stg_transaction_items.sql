with source as (
    select * from {{ source('raw', 'transaction_items') }} final
)
select
    {{ gen_sk(['tenant_id', 'item_id']) }}        as item_sk,
    tenant_id,
    item_id,
    transaction_id,
    {{ gen_sk(['tenant_id', 'transaction_id']) }} as transaction_sk,
    product_id,
    {{ gen_sk(['tenant_id', 'product_id']) }}     as product_sk,
    quantity,
    unit_price,
    discount,
    subtotal,
    created_at,
    updated_at
from source
