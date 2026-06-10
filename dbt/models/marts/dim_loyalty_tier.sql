-- Static conformed dimension describing the loyalty tiers and their ranking.
with tiers as (
    select 'bronze'   as tier, 1 as tier_rank, toFloat64(0)       as min_spend
    union all select 'silver',   2, toFloat64(500000)
    union all select 'gold',     3, toFloat64(1500000)
    union all select 'platinum', 4, toFloat64(3000000)
)
select
    tier        as tier_sk,
    tier,
    tier_rank,
    min_spend,
    {{ audit_columns() }}
from tiers
