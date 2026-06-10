with source as (
    select * from {{ source('raw', 'customers') }} final
)
select
    {{ gen_sk(['tenant_id', 'customer_id']) }} as customer_sk,
    tenant_id,
    customer_id,
    trim(name)                       as customer_name,
    phone,
    lower(email)                     as email,
    gender,
    city,
    created_at,
    updated_at
from source
