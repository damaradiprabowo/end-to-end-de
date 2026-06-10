-- Advanced intermediate layer: complex join performed BEFORE the fact tables.
-- Grain: one row per transaction line item, enriched with product, store,
-- employee and primary-supplier cost so downstream marts/analytics can compute
-- margin, employee performance and supplier dependency without re-joining.
with items as (
    select * from {{ ref('stg_transaction_items') }}
),
txns as (
    select * from {{ ref('stg_transactions') }}
),
products as (
    select * from {{ ref('stg_products') }}
),
stores as (
    select * from {{ ref('stg_stores') }}
),
employees as (
    select * from {{ ref('stg_employees') }}
),
-- primary supplier cost per product (one row per product_sk)
primary_cost as (
    select
        product_sk,
        anyLast(supplier_sk) as primary_supplier_sk,
        anyLast(cost_price)  as cost_price
    from {{ ref('stg_product_supplier') }}
    where is_primary = true
    group by product_sk
)
-- NB: joined columns are explicitly aliased so ClickHouse does not retain the
-- table qualifier in the output column name for join-ambiguous names (e.g.
-- tenant_id / transaction_sk / store_sk). See fact_sales.sql for detail.
select
    i.item_sk                                   as sales_sk,
    i.tenant_id                                 as tenant_id,
    i.transaction_sk                            as transaction_sk,
    i.transaction_id                            as transaction_id,
    t.date_key                                  as date_key,
    t.transaction_date                          as transaction_date,
    t.customer_sk                               as customer_sk,
    t.store_sk                                  as store_sk,
    s.city                                      as store_city,
    t.employee_sk                               as employee_sk,
    e.employee_name                             as employee_name,
    e.role                                      as employee_role,
    i.product_sk                                as product_sk,
    p.product_name                              as product_name,
    p.category                                  as category,
    pc.primary_supplier_sk                      as primary_supplier_sk,
    i.quantity                                  as quantity,
    i.unit_price                                as unit_price,
    i.subtotal                                  as net_amount,
    i.quantity * i.unit_price                   as gross_amount,
    coalesce(pc.cost_price, 0)                  as cost_price,
    i.quantity * coalesce(pc.cost_price, 0)     as cost_amount,
    i.subtotal - (i.quantity * coalesce(pc.cost_price, 0)) as margin_amount,
    {{ audit_columns() }}
from items i
inner join txns t      on i.transaction_sk = t.transaction_sk
left join products p   on i.product_sk = p.product_sk
left join stores s     on t.store_sk = s.store_sk
left join employees e  on t.employee_sk = e.employee_sk
left join primary_cost pc on i.product_sk = pc.product_sk
