with source as (
    select * from {{ source('raw', 'customer_loyalty') }} final
)
select
    {{ gen_sk(['tenant_id', 'loyalty_id']) }}  as loyalty_sk,
    tenant_id,
    loyalty_id,
    {{ gen_sk(['tenant_id', 'customer_id']) }} as customer_sk,
    customer_id,
    lower(tier)                                as tier,
    points,
    total_spend,
    member_since,
    created_at,
    updated_at
from source
