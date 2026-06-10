with source as (
    select * from {{ source('raw', 'promotions') }} final
)
select
    {{ gen_sk(['tenant_id', 'promo_id']) }} as promo_sk,
    tenant_id,
    promo_id,
    promo_name,
    promo_type,
    discount_pct,
    start_date,
    end_date,
    min_purchase,
    created_at,
    updated_at
from source
