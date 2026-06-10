-- Date dimension generated from the min/max transaction date in the data.
with bounds as (
    select
        toDate(min(transaction_date)) as start_date,
        toDate(max(transaction_date)) as end_date
    from {{ ref('stg_transactions') }}
),
spine as (
    -- bounds has exactly one row; ARRAY JOIN over range() expands it to one
    -- row per day across the full [start_date, end_date] range.
    select
        start_date + toIntervalDay(number) as date_day
    from bounds
    array join range(toUInt32(end_date - start_date + 1)) as number
)
select
    toInt32(formatDateTime(date_day, '%Y%m%d')) as date_key,
    date_day,
    toYear(date_day)                            as year,
    toQuarter(date_day)                         as quarter,
    toMonth(date_day)                           as month,
    formatDateTime(date_day, '%Y-%m')           as year_month,
    formatDateTime(date_day, '%b')              as month_name,
    toDayOfMonth(date_day)                      as day_of_month,
    toDayOfWeek(date_day)                       as day_of_week,
    formatDateTime(date_day, '%a')              as day_name,
    if(toDayOfWeek(date_day) >= 6, 1, 0)        as is_weekend,
    {{ audit_columns() }}
from spine
