with promotions as (
    select * from {{ ref('stg_promotions') }}
)
select
    promo_sk,
    tenant_id,
    promo_id,
    promo_name,
    promo_type,
    discount_pct,
    start_date,
    end_date,
    min_purchase,
    {{ audit_columns() }}
from promotions
