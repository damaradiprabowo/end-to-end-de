with suppliers as (
    select * from {{ ref('stg_suppliers') }}
)
select
    supplier_sk,
    tenant_id,
    supplier_id,
    supplier_name,
    contact_name,
    city,
    country,
    {{ audit_columns() }}
from suppliers
