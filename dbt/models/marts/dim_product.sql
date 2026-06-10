with products as (
    select * from {{ ref('stg_products') }}
)
select
    product_sk,
    tenant_id,
    product_id,
    product_name,
    category,
    brand,
    unit_price,
    is_active,
    {{ audit_columns() }}
from products
