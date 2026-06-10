-- Grain: one row per transaction line item (completed transactions only).
-- Enriched with store, customer, date, payment and transaction-level promo info.
with items as (
    select * from {{ ref('stg_transaction_items') }}
),
txns as (
    select * from {{ ref('stg_transactions') }}
),
promo_agg as (
    select
        transaction_sk,
        sum(discount_applied) as promo_discount_total,
        count()               as promo_count
    from {{ ref('stg_transaction_promotions') }}
    group by transaction_sk
)
-- NB: every joined column carries an explicit alias. ClickHouse keeps the
-- table qualifier in the *output* column name for any name that is ambiguous
-- across the join (e.g. tenant_id / transaction_sk exist in both items and
-- txns), which would otherwise emit a column literally named `i.transaction_sk`
-- and break downstream references.
select
    i.item_sk                              as sales_sk,
    i.tenant_id                            as tenant_id,
    i.transaction_sk                       as transaction_sk,
    i.transaction_id                       as transaction_id,
    t.date_key                             as date_key,
    t.transaction_date                     as transaction_date,
    t.customer_sk                          as customer_sk,
    t.store_sk                             as store_sk,
    t.employee_sk                          as employee_sk,
    i.product_sk                           as product_sk,
    i.product_id                           as product_id,
    i.quantity                             as quantity,
    i.unit_price                           as unit_price,
    i.discount                             as line_discount_pct,
    i.quantity * i.unit_price              as gross_amount,
    i.subtotal                             as net_amount,
    t.payment_method                       as payment_method,
    -- transaction-level (repeated per line item; do not sum across lines)
    coalesce(pa.promo_discount_total, 0)   as txn_promo_discount,
    if(pa.promo_count > 0, 1, 0)           as is_promo_transaction,
    {{ audit_columns() }}
from items i
inner join txns t  on i.transaction_sk = t.transaction_sk
left join promo_agg pa on i.transaction_sk = pa.transaction_sk
