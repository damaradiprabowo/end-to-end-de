-- Mart DQ: metric consistency.
-- Total revenue in fact_sales must reconcile (within a rounding tolerance)
-- with the revenue derived from the staging line items of completed
-- transactions. Any returned row fails the test.
with fact as (
    select round(sum(net_amount), 2) as revenue from {{ ref('fact_sales') }}
),
staged as (
    select round(sum(i.subtotal), 2) as revenue
    from {{ ref('stg_transaction_items') }} i
    inner join {{ ref('stg_transactions') }} t
        on i.transaction_sk = t.transaction_sk
)
select
    fact.revenue   as fact_revenue,
    staged.revenue as staged_revenue,
    abs(fact.revenue - staged.revenue) as diff
from fact, staged
where abs(fact.revenue - staged.revenue) > 1.0
