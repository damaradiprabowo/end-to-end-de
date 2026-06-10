-- Fact (snapshot grain): one row per customer loyalty record, capturing the
-- accumulated points, current tier and recorded total spend.
with loyalty as (
    select * from {{ ref('stg_customer_loyalty') }}
)
select
    loyalty_sk,
    tenant_id,
    customer_sk,
    customer_id,
    tier        as tier_sk,
    tier,
    points,
    total_spend as recorded_total_spend,
    member_since,
    {{ audit_columns() }}
from loyalty
