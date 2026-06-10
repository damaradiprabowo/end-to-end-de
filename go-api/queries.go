package main

// SQL for the 7 Advanced analytic questions (brief §8.3), served under the
// `/api/v1/*` routes the brief specifies (§8.4) from the ClickHouse `analytics`
// star schema plus the enriched intermediate layer.
//
// Each query is run through the ClickHouse HTTP interface with FORMAT JSON, so
// every endpoint returns a flat array of objects ready for the dashboard. Every
// question touches at least one fact table; Q1/Q3/Q5/Q6 combine two or more
// (fact_sales + the enriched intermediate / inventory / promotion / supplier
// data) to satisfy the "min. 3 fact" requirement across the set.

// Q1 — Revenue & estimated margin per product category per quarter.
// margin = revenue − (cost_price × qty); cost is already joined in the enriched
// intermediate layer (primary-supplier cost), so no re-join is needed here.
const qRevenueMargin = `
SELECT
    e.category AS category,
    concat(toString(d.year), '-Q', toString(d.quarter)) AS quarter,
    round(sum(e.net_amount), 2)    AS revenue,
    round(sum(e.margin_amount), 2) AS margin,
    round(100.0 * sum(e.margin_amount) / nullIf(sum(e.net_amount), 0), 1) AS margin_pct
FROM analytics.int_sales_enriched AS e
JOIN analytics.dim_date AS d ON e.date_key = d.date_key
WHERE e.category != ''
GROUP BY category, quarter
ORDER BY quarter, revenue DESC`

// Q2 — Customer Lifetime Value: top 20 customers by total spend across the whole
// period, with their current loyalty tier. Loyalty is collapsed to one row per
// customer so the LEFT JOIN cannot fan out the pre-aggregated spend.
const qCustomerLTV = `
WITH spend AS (
    SELECT
        customer_sk,
        sum(net_amount)                AS lifetime_value,
        count(DISTINCT transaction_sk) AS transactions
    FROM analytics.fact_sales
    GROUP BY customer_sk
),
tier AS (
    SELECT customer_sk, anyLast(tier) AS tier
    FROM analytics.fact_loyalty_activity
    GROUP BY customer_sk
)
SELECT
    c.customer_name            AS customer_name,
    c.city                     AS city,
    coalesce(t.tier, 'none')   AS tier,
    round(s.lifetime_value, 2) AS lifetime_value,
    s.transactions             AS transactions
FROM spend AS s
JOIN analytics.dim_customer AS c ON s.customer_sk = c.customer_sk
LEFT JOIN tier AS t ON s.customer_sk = t.customer_sk
ORDER BY lifetime_value DESC
LIMIT 20`

// Q3 — Inventory turnover per store×product (qty_sold / avg_stock). The turnover
// rate is pre-computed in fact_inventory_movement (which already joins units sold
// from the enriched layer to the inventory snapshot). Lowest first surfaces the
// slow-moving stock the question asks to identify.
const qInventoryTurnover = `
SELECT
    st.store_name              AS store_name,
    p.product_name             AS product_name,
    im.stock_qty               AS stock_qty,
    im.qty_sold                AS qty_sold,
    round(im.turnover_rate, 3) AS turnover_rate
FROM analytics.fact_inventory_movement AS im
JOIN analytics.dim_store   AS st ON im.store_sk = st.store_sk
JOIN analytics.dim_product AS p  ON im.product_sk = p.product_sk
WHERE im.stock_qty > 0
ORDER BY turnover_rate ASC
LIMIT 20`

// Q4 — Employee effectiveness: average transaction value per cashier (plus
// revenue and margin) in each store. Sourced from the enriched intermediate
// layer, which carries employee + cost so margin needs no re-join.
const qEmployeePerformance = `
SELECT
    employee_name                  AS employee_name,
    employee_role                  AS role,
    store_city                     AS store_city,
    round(sum(net_amount), 2)      AS revenue,
    round(sum(margin_amount), 2)   AS margin,
    count(DISTINCT transaction_sk) AS transactions,
    round(sum(net_amount) / nullIf(count(DISTINCT transaction_sk), 0), 2) AS avg_txn_value
FROM analytics.int_sales_enriched
WHERE employee_name != ''
GROUP BY employee_name, employee_role, store_city
ORDER BY avg_txn_value DESC
LIMIT 15`

// Q5 — Promotion ROI: (revenue while promo active − baseline revenue) / total
// discount. Baseline = average value of a non-promo transaction; incremental
// revenue is the per-promo transaction count times the lift over that baseline.
// Rows are deduped to one (promo, transaction) pair first so a transaction with
// several promos does not double-count its amount.
const qPromotionROI = `
WITH per_txn AS (
    SELECT
        promo_sk,
        transaction_sk,
        any(transaction_amount) AS txn_amount,
        sum(discount_applied)   AS discount
    FROM analytics.fact_promotion_usage
    GROUP BY promo_sk, transaction_sk
),
agg AS (
    SELECT
        promo_sk,
        sum(txn_amount) AS promo_revenue,
        sum(discount)   AS total_discount,
        count()         AS promo_txns,
        avg(txn_amount) AS avg_promo_txn
    FROM per_txn
    GROUP BY promo_sk
),
baseline AS (
    SELECT avg(txn_amount) AS baseline_avg
    FROM (
        SELECT transaction_sk, sum(net_amount) AS txn_amount
        FROM analytics.fact_sales
        WHERE is_promo_transaction = 0
        GROUP BY transaction_sk
    )
)
SELECT
    p.promo_name               AS promo_name,
    p.promo_type               AS promo_type,
    round(a.promo_revenue, 2)  AS promo_revenue,
    round(a.total_discount, 2) AS total_discount,
    a.promo_txns               AS promo_txns,
    round((a.avg_promo_txn - b.baseline_avg) * a.promo_txns, 2) AS incremental_revenue,
    round(((a.avg_promo_txn - b.baseline_avg) * a.promo_txns) / nullIf(a.total_discount, 0), 2) AS roi
FROM agg AS a
JOIN analytics.dim_promotion AS p ON a.promo_sk = p.promo_sk
CROSS JOIN baseline AS b
ORDER BY roi DESC`

// Q6 — Supplier dependency for the top-selling products: how many distinct
// suppliers each top seller can be sourced from (single-source = a supply risk,
// multi-source = resilient). Combines fact_sales (to rank top sellers) with the
// product_supplier bridge.
const qSupplierDependency = `
WITH top_products AS (
    SELECT
        product_sk,
        sum(net_amount) AS revenue,
        sum(quantity)   AS qty_sold
    FROM analytics.fact_sales
    GROUP BY product_sk
    ORDER BY revenue DESC
    LIMIT 20
),
supplier_count AS (
    SELECT product_sk, count(DISTINCT supplier_sk) AS supplier_count
    FROM analytics.stg_product_supplier
    GROUP BY product_sk
)
SELECT
    p.product_name                 AS product_name,
    p.category                     AS category,
    round(t.revenue, 2)            AS revenue,
    t.qty_sold                     AS qty_sold,
    coalesce(sc.supplier_count, 0) AS supplier_count,
    if(coalesce(sc.supplier_count, 0) <= 1, 'single-source', 'multi-source') AS sourcing
FROM top_products AS t
JOIN analytics.dim_product AS p ON t.product_sk = p.product_sk
LEFT JOIN supplier_count AS sc ON t.product_sk = sc.product_sk
ORDER BY revenue DESC`

// Q7 — Monthly customer-retention cohort. Each customer is assigned to the cohort
// of their first purchase month; retention_pct is the share of that cohort still
// transacting `month_offset` months later. Rendered as a heatmap on the dashboard.
const qRetentionCohort = `
WITH first_month AS (
    SELECT customer_sk, toStartOfMonth(min(transaction_date)) AS cohort_month
    FROM analytics.fact_sales
    GROUP BY customer_sk
),
activity AS (
    SELECT DISTINCT customer_sk, toStartOfMonth(transaction_date) AS active_month
    FROM analytics.fact_sales
),
joined AS (
    SELECT
        fm.cohort_month                                    AS cohort_month,
        dateDiff('month', fm.cohort_month, a.active_month) AS month_offset,
        a.customer_sk                                      AS customer_sk
    FROM first_month AS fm
    JOIN activity AS a ON fm.customer_sk = a.customer_sk
),
cohort_size AS (
    SELECT cohort_month, count(DISTINCT customer_sk) AS size
    FROM first_month
    GROUP BY cohort_month
)
SELECT
    formatDateTime(j.cohort_month, '%Y-%m')  AS cohort_month,
    j.month_offset                           AS month_offset,
    cs.size                                  AS cohort_size,
    count(DISTINCT j.customer_sk)            AS active_customers,
    round(100.0 * count(DISTINCT j.customer_sk) / cs.size, 1) AS retention_pct
FROM joined AS j
JOIN cohort_size AS cs ON j.cohort_month = cs.cohort_month
WHERE j.month_offset >= 0
GROUP BY cohort_month, month_offset, cs.size
ORDER BY cohort_month, month_offset`

// endpoint binds a route to its SQL and a short description (used by the index).
type endpoint struct {
	path string
	sql  string
	desc string
}

// The 7 Advanced endpoints, paths per brief §8.4.
var endpoints = []endpoint{
	{"/api/v1/revenue-margin", qRevenueMargin, "Q1 — revenue & estimated margin per category per quarter"},
	{"/api/v1/customer-ltv", qCustomerLTV, "Q2 — top-20 customers by lifetime value + loyalty tier"},
	{"/api/v1/inventory-turnover", qInventoryTurnover, "Q3 — inventory turnover per store×product (slowest first)"},
	{"/api/v1/employee-performance", qEmployeePerformance, "Q4 — avg transaction value, revenue & margin per cashier"},
	{"/api/v1/promotion-roi", qPromotionROI, "Q5 — promotion ROI: incremental revenue / total discount"},
	{"/api/v1/supplier-dependency", qSupplierDependency, "Q6 — single- vs multi-source dependency for top sellers"},
	{"/api/v1/cohort-retention", qRetentionCohort, "Q7 — monthly customer-retention cohort (heatmap)"},
}
