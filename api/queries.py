"""SQL for the 5 Intermediate analytic questions (ClickHouse / analytics db)."""

# Q1 — Revenue per store per month (last 6 months of available data)
REVENUE_BY_STORE = """
SELECT
    s.store_name AS store_name,
    s.city       AS city,
    d.year_month AS year_month,
    round(sum(f.net_amount), 2) AS revenue
FROM analytics.fact_sales AS f
JOIN analytics.dim_store AS s ON f.store_sk = s.store_sk
JOIN analytics.dim_date  AS d ON f.date_key = d.date_key
WHERE d.date_day >= addMonths((SELECT max(date_day) FROM analytics.dim_date), -6)
GROUP BY store_name, city, year_month
ORDER BY year_month, store_name
"""

# Q2 — Promotion effectiveness: discount totals + avg transaction value
PROMOTION_EFFECTIVENESS = """
WITH promo_txn AS (
    SELECT
        promo_sk,
        round(sum(discount_applied), 2)       AS total_discount,
        count(DISTINCT transaction_sk)         AS transactions,
        round(avg(transaction_amount), 2)      AS avg_txn_value
    FROM analytics.fact_promotion_usage
    GROUP BY promo_sk
)
SELECT
    p.promo_name AS promo_name,
    p.promo_type AS promo_type,
    pt.total_discount,
    pt.transactions,
    pt.avg_txn_value
FROM promo_txn AS pt
JOIN analytics.dim_promotion AS p ON pt.promo_sk = p.promo_sk
ORDER BY pt.total_discount DESC
"""

PROMO_AVG_COMPARISON = """
SELECT
    if(is_promo_transaction = 1, 'with_promo', 'without_promo') AS bucket,
    round(avg(txn_amount), 2) AS avg_transaction_value,
    count() AS transactions
FROM (
    SELECT
        transaction_sk,
        any(is_promo_transaction) AS is_promo_transaction,
        sum(net_amount)           AS txn_amount
    FROM analytics.fact_sales
    GROUP BY transaction_sk
)
GROUP BY bucket
"""

# Q3 — Top 3 products by revenue in each city
TOP_PRODUCTS_BY_CITY = """
SELECT city, product_name, revenue
FROM (
    SELECT
        s.city          AS city,
        p.product_name  AS product_name,
        round(sum(f.net_amount), 2) AS revenue,
        row_number() OVER (PARTITION BY s.city ORDER BY sum(f.net_amount) DESC) AS rn
    FROM analytics.fact_sales AS f
    JOIN analytics.dim_store   AS s ON f.store_sk = s.store_sk
    JOIN analytics.dim_product AS p ON f.product_sk = p.product_sk
    GROUP BY city, product_name
)
WHERE rn <= 3
ORDER BY city, revenue DESC
"""

# Q4 — Customer spending segmentation (High/Medium/Low) per city
CUSTOMER_SEGMENTS = """
WITH cust AS (
    SELECT
        f.customer_sk          AS customer_sk,
        c.city                 AS city,
        sum(f.net_amount)      AS total_spend
    FROM analytics.fact_sales AS f
    JOIN analytics.dim_customer AS c ON f.customer_sk = c.customer_sk
    GROUP BY customer_sk, city
),
thresholds AS (
    SELECT quantile(0.66)(total_spend) AS q_high,
           quantile(0.33)(total_spend) AS q_low
    FROM cust
),
seg AS (
    SELECT
        city,
        customer_sk,
        total_spend,
        multiIf(total_spend >= q_high, 'High',
                total_spend >= q_low,  'Medium',
                'Low') AS segment
    FROM cust CROSS JOIN thresholds
)
SELECT
    city,
    segment,
    count()                       AS customers,
    round(sum(total_spend), 2)    AS total_spend
FROM seg
GROUP BY city, segment
ORDER BY city, segment
"""

# Q5 — Transactions and revenue by day of week
TRANSACTIONS_BY_DAY = """
SELECT
    d.day_of_week               AS day_of_week,
    d.day_name                  AS day_name,
    count(DISTINCT f.transaction_sk) AS transactions,
    round(sum(f.net_amount), 2) AS revenue
FROM analytics.fact_sales AS f
JOIN analytics.dim_date AS d ON f.date_key = d.date_key
GROUP BY day_of_week, day_name
ORDER BY day_of_week
"""
