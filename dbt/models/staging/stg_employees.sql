with source as (
    select * from {{ source('raw', 'employees') }} final
)
select
    {{ gen_sk(['tenant_id', 'employee_id']) }} as employee_sk,
    tenant_id,
    employee_id,
    {{ gen_sk(['tenant_id', 'store_id']) }}    as store_sk,
    store_id,
    trim(name)                                 as employee_name,
    lower(role)                                as role,
    hire_date,
    is_active,
    created_at,
    updated_at
from source
