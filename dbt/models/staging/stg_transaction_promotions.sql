with source as (
    select * from {{ source('raw', 'transaction_promotions') }} final
)
select
    {{ gen_sk(['tenant_id', 'id']) }}             as txn_promo_sk,
    tenant_id,
    id                                            as txn_promo_id,
    transaction_id,
    {{ gen_sk(['tenant_id', 'transaction_id']) }} as transaction_sk,
    promo_id,
    {{ gen_sk(['tenant_id', 'promo_id']) }}       as promo_sk,
    discount_applied,
    created_at,
    updated_at
from source
