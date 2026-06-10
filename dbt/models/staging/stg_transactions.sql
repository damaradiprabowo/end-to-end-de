with source as (
    select * from {{ source('raw', 'transactions') }} final
)
select
    {{ gen_sk(['tenant_id', 'transaction_id']) }} as transaction_sk,
    tenant_id,
    transaction_id,
    {{ gen_sk(['tenant_id', 'customer_id']) }}    as customer_sk,
    {{ gen_sk(['tenant_id', 'store_id']) }}       as store_sk,
    {{ gen_sk(['tenant_id', 'employee_id']) }}    as employee_sk,
    customer_id,
    store_id,
    employee_id,
    transaction_date,
    {{ date_key('transaction_date') }}            as date_key,
    total_amount,
    lower(payment_method)                         as payment_method,
    status,
    created_at,
    updated_at
from source
-- Beginner spec: keep only completed transactions
where status = 'completed'
