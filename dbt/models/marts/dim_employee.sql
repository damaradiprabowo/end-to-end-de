with employees as (
    select * from {{ ref('stg_employees') }}
)
select
    employee_sk,
    tenant_id,
    employee_id,
    store_sk,
    store_id,
    employee_name,
    role,
    hire_date,
    is_active,
    {{ audit_columns() }}
from employees
