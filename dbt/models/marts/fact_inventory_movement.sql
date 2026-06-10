-- Fact: inventory position and movement per store x product.
-- Combines the inventory snapshot (stock in) with units sold (stock out) so
-- turnover can be derived (qty_sold / avg_stock).
with inv as (
    select * from {{ ref('stg_inventory') }}
),
sold as (
    select
        tenant_id,
        store_sk,
        product_sk,
        sum(quantity) as qty_sold
    from {{ ref('int_sales_enriched') }}
    group by tenant_id, store_sk, product_sk
)
-- inv.* columns are explicitly aliased: tenant_id / store_sk / product_sk are
-- ambiguous against the joined `sold` aggregate, so ClickHouse would otherwise
-- keep the `inv.` qualifier in the output column name.
select
    inv.inventory_sk    as inventory_sk,
    inv.tenant_id       as tenant_id,
    inv.store_sk        as store_sk,
    inv.product_sk      as product_sk,
    inv.supplier_sk     as supplier_sk,
    inv.stock_qty       as stock_qty,
    inv.reorder_level   as reorder_level,
    coalesce(sold.qty_sold, 0)                              as qty_sold,
    -- turnover rate using current stock as the average-stock proxy
    if(inv.stock_qty > 0,
       round(coalesce(sold.qty_sold, 0) / inv.stock_qty, 4),
       0)                                                   as turnover_rate,
    if(inv.stock_qty <= inv.reorder_level, 1, 0)            as below_reorder,
    inv.last_restocked  as last_restocked,
    {{ audit_columns() }}
from inv
left join sold
    on inv.tenant_id = sold.tenant_id
   and inv.store_sk  = sold.store_sk
   and inv.product_sk = sold.product_sk
