-- Grain: one row per promotion applied to a (completed) transaction.
with tp as (
    select * from {{ ref('stg_transaction_promotions') }}
),
txns as (
    select * from {{ ref('stg_transactions') }}
)
-- Joined columns are explicitly aliased so ClickHouse does not keep the table
-- qualifier in the output name for join-ambiguous columns (tenant_id /
-- transaction_sk / transaction_id exist in both tp and txns).
select
    tp.txn_promo_sk      as txn_promo_sk,
    tp.tenant_id         as tenant_id,
    tp.transaction_sk    as transaction_sk,
    tp.transaction_id    as transaction_id,
    tp.promo_sk          as promo_sk,
    t.store_sk           as store_sk,
    t.customer_sk        as customer_sk,
    t.date_key           as date_key,
    t.transaction_date   as transaction_date,
    tp.discount_applied  as discount_applied,
    t.total_amount       as transaction_amount,
    {{ audit_columns() }}
from tp
inner join txns t on tp.transaction_sk = t.transaction_sk
