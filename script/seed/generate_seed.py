"""
Seed generator for the Minimarket POS OLTP database.

Populates every tenant schema (tenant_01, tenant_02, tenant_03) with
realistic, internally-consistent fake data:
  stores -> suppliers -> products -> promotions -> customers
  -> transactions -> transaction_items -> transaction_promotions

Re-runnable: it TRUNCATEs the tenant tables before inserting.
Configuration is read from environment variables (see .env.example).
"""
import os
import random
import logging
from datetime import datetime, timedelta, date

import psycopg2
from psycopg2.extras import execute_values
from faker import Faker

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)-7s | seed | %(message)s",
)
log = logging.getLogger("seed")

fake = Faker("id_ID")
Faker.seed(42)
random.seed(42)

# ----------------------------- config -----------------------------
PG = dict(
    host=os.getenv("POSTGRES_HOST", "localhost"),
    port=int(os.getenv("POSTGRES_PORT", "5432")),
    dbname=os.getenv("POSTGRES_DB", "pos"),
    user=os.getenv("POSTGRES_USER", "pos_user"),
    password=os.getenv("POSTGRES_PASSWORD", "pos_password"),
)
TENANTS = os.getenv("TENANT_SCHEMAS", "tenant_01,tenant_02,tenant_03").split(",")
N_CUSTOMERS = int(os.getenv("SEED_CUSTOMERS", "200"))
N_PRODUCTS = int(os.getenv("SEED_PRODUCTS", "120"))
N_TRANSACTIONS = int(os.getenv("SEED_TRANSACTIONS", "3000"))

CITIES = ["Jakarta", "Bandung", "Surabaya", "Medan", "Semarang", "Yogyakarta", "Makassar"]
PROVINCES = {
    "Jakarta": "DKI Jakarta", "Bandung": "Jawa Barat", "Surabaya": "Jawa Timur",
    "Medan": "Sumatera Utara", "Semarang": "Jawa Tengah", "Yogyakarta": "DI Yogyakarta",
    "Makassar": "Sulawesi Selatan",
}
CATEGORIES = {
    "Beverages": ["Aqua", "Teh Botol", "Coca Cola", "Pocari"],
    "Snacks": ["Chitato", "Lays", "Taro", "Qtela"],
    "Dairy": ["Ultra", "Indomilk", "Frisian Flag", "Cimory"],
    "Personal Care": ["Lifebuoy", "Pepsodent", "Rejoice", "Gillette"],
    "Household": ["Rinso", "Sunlight", "Baygon", "Stella"],
    "Instant Food": ["Indomie", "Sarimi", "Pop Mie", "Sedaap"],
}
PAYMENTS = ["cash", "debit", "credit", "e-wallet"]
STORE_TYPES = ["minimarket", "supermarket", "express"]
PROMO_TYPES = ["discount", "bundle", "cashback"]


ROLES = ["cashier", "cashier", "cashier", "supervisor", "manager"]
LOYALTY_TIERS = ["bronze", "silver", "gold", "platinum"]


def truncate(cur, schema):
    tables = [
        # children first
        "customer_loyalty", "product_supplier", "inventory",
        "transaction_promotions", "transaction_items", "transactions",
        "employees", "promotions", "products", "customers",
        "suppliers", "stores",
    ]
    for t in tables:
        cur.execute(f'TRUNCATE TABLE "{schema}".{t} RESTART IDENTITY CASCADE;')
    log.info("[%s] truncated existing data", schema)


def seed_tenant(cur, schema, tenant_idx):
    truncate(cur, schema)
    base_city = CITIES[tenant_idx % len(CITIES)]

    # -------- stores (2-3 per tenant) --------
    n_stores = random.randint(2, 3)
    stores = []
    for i in range(n_stores):
        city = base_city if i == 0 else random.choice(CITIES)
        stores.append((
            f"{schema.upper()} Mart {city} #{i+1}", city, PROVINCES[city],
            random.choice(STORE_TYPES),
            fake.date_between(start_date="-5y", end_date="-1y"), True,
        ))
    store_ids = execute_values(cur, f'''
        INSERT INTO "{schema}".stores
        (store_name, city, province, store_type, opened_at, is_active)
        VALUES %s RETURNING store_id''', stores, fetch=True)
    store_ids = [r[0] for r in store_ids]

    # -------- employees (3-6 per store) --------
    employees = []
    for sid in store_ids:
        for _ in range(random.randint(3, 6)):
            employees.append((
                sid, fake.name(), random.choice(ROLES),
                fake.date_between(start_date="-4y", end_date="-1m"), True,
            ))
    emp_rows = execute_values(cur, f'''
        INSERT INTO "{schema}".employees
        (store_id, name, role, hire_date, is_active)
        VALUES %s RETURNING employee_id, store_id''', employees, fetch=True)
    store_employees = {}
    for emp_id, sid in emp_rows:
        store_employees.setdefault(sid, []).append(emp_id)

    # -------- suppliers --------
    suppliers = [(
        fake.company(), fake.name(), random.choice(CITIES), "Indonesia",
    ) for _ in range(random.randint(5, 10))]
    supplier_rows = execute_values(cur, f'''
        INSERT INTO "{schema}".suppliers
        (supplier_name, contact_name, city, country)
        VALUES %s RETURNING supplier_id''', suppliers, fetch=True)
    supplier_ids = [r[0] for r in supplier_rows]

    # -------- products --------
    products = []
    for _ in range(N_PRODUCTS):
        cat = random.choice(list(CATEGORIES.keys()))
        brand = random.choice(CATEGORIES[cat])
        products.append((
            f"{brand} {fake.word().capitalize()}", cat, brand,
            round(random.uniform(2000, 75000), 2), True,
        ))
    product_rows = execute_values(cur, f'''
        INSERT INTO "{schema}".products
        (product_name, category, brand, unit_price, is_active)
        VALUES %s RETURNING product_id, unit_price''', products, fetch=True)
    product_ids = [r[0] for r in product_rows]
    price_map = {r[0]: float(r[1]) for r in product_rows}

    # -------- product_supplier (1-3 suppliers per product, one primary) --------
    prod_sup = []
    primary_cost = {}
    for pid in product_ids:
        chosen = random.sample(supplier_ids, min(random.randint(1, 3), len(supplier_ids)))
        for j, sup_id in enumerate(chosen):
            cost = round(price_map[pid] * random.uniform(0.55, 0.8), 2)
            is_primary = (j == 0)
            if is_primary:
                primary_cost[pid] = (sup_id, cost)
            prod_sup.append((pid, sup_id, cost, random.randint(1, 21), is_primary))
    execute_values(cur, f'''
        INSERT INTO "{schema}".product_supplier
        (product_id, supplier_id, cost_price, lead_time_days, is_primary)
        VALUES %s''', prod_sup)

    # -------- promotions --------
    promos = []
    for _ in range(random.randint(6, 12)):
        start = fake.date_between(start_date="-1y", end_date="-1m")
        promos.append((
            fake.catch_phrase()[:90], random.choice(PROMO_TYPES),
            round(random.uniform(5, 40), 2), start,
            start + timedelta(days=random.randint(7, 60)),
            round(random.choice([0, 25000, 50000, 100000]), 2),
        ))
    promo_rows = execute_values(cur, f'''
        INSERT INTO "{schema}".promotions
        (promo_name, promo_type, discount_pct, start_date, end_date, min_purchase)
        VALUES %s RETURNING promo_id''', promos, fetch=True)
    promo_ids = [r[0] for r in promo_rows]

    # -------- customers --------
    customers = []
    for _ in range(N_CUSTOMERS):
        g = random.choice(["Male", "Female"])
        customers.append((
            fake.name_male() if g == "Male" else fake.name_female(),
            fake.phone_number()[:20], fake.email(), g, random.choice(CITIES),
        ))
    customer_rows = execute_values(cur, f'''
        INSERT INTO "{schema}".customers (name, phone, email, gender, city)
        VALUES %s RETURNING customer_id''', customers, fetch=True)
    customer_ids = [r[0] for r in customer_rows]

    # -------- transactions + items + promo usage --------
    start_window = datetime.now() - timedelta(days=270)
    tx_batch, item_batch, txpromo_batch = [], [], []
    # We need transaction ids; insert transactions first, then items.
    for _ in range(N_TRANSACTIONS):
        tx_date = start_window + timedelta(
            seconds=random.randint(0, 270 * 24 * 3600))
        store_id = random.choice(store_ids)
        emp_pool = store_employees.get(store_id) or [None]
        tx_batch.append((
            random.choice(customer_ids), store_id, random.choice(emp_pool), tx_date,
            random.choice(PAYMENTS),
            "completed" if random.random() > 0.05 else "cancelled",
        ))
    tx_rows = execute_values(cur, f'''
        INSERT INTO "{schema}".transactions
        (customer_id, store_id, employee_id, transaction_date, payment_method, status, total_amount)
        VALUES %s RETURNING transaction_id, transaction_date''',
        [(c, s, e, d, p, st, 0) for (c, s, e, d, p, st) in tx_batch], fetch=True)

    # build items and update totals (tx_rows preserves tx_batch order)
    totals = {}
    customer_spend = {}            # customer_id -> total completed spend
    qty_sold = {}                  # (store_id, product_id) -> total qty
    for (cust_id, store_id, _emp, _d, _p, status), (tx_id, _tx_date) in zip(tx_batch, tx_rows):
        n_items = random.randint(1, 6)
        chosen = random.sample(product_ids, min(n_items, len(product_ids)))
        total = 0.0
        for pid in chosen:
            qty = random.randint(1, 5)
            price = price_map[pid]
            disc = random.choice([0, 0, 0, 5, 10])
            subtotal = round(qty * price * (1 - disc / 100.0), 2)
            total += subtotal
            item_batch.append((tx_id, pid, qty, price, disc, subtotal))
            if status == "completed":
                qty_sold[(store_id, pid)] = qty_sold.get((store_id, pid), 0) + qty
        totals[tx_id] = round(total, 2)
        if status == "completed":
            customer_spend[cust_id] = customer_spend.get(cust_id, 0.0) + total
        # ~25% of transactions use a promotion
        if random.random() < 0.25 and promo_ids:
            applied = round(total * random.uniform(0.03, 0.15), 2)
            txpromo_batch.append((tx_id, random.choice(promo_ids), applied))

    execute_values(cur, f'''
        INSERT INTO "{schema}".transaction_items
        (transaction_id, product_id, quantity, unit_price, discount, subtotal)
        VALUES %s''', item_batch)
    # update totals in one pass
    execute_values(cur, f'''
        UPDATE "{schema}".transactions AS t SET total_amount = v.total
        FROM (VALUES %s) AS v(tx_id, total)
        WHERE t.transaction_id = v.tx_id''',
        list(totals.items()))
    if txpromo_batch:
        execute_values(cur, f'''
            INSERT INTO "{schema}".transaction_promotions
            (transaction_id, promo_id, discount_applied) VALUES %s''', txpromo_batch)

    # -------- inventory (one row per store x product) --------
    inventory = []
    for store_id in store_ids:
        for pid in product_ids:
            sold = qty_sold.get((store_id, pid), 0)
            reorder = random.randint(5, 20)
            # stock loosely related to sales so turnover is meaningful
            stock = max(reorder, int(sold * random.uniform(0.3, 1.5)) + random.randint(0, 30))
            sup_id = primary_cost.get(pid, (random.choice(supplier_ids), 0))[0]
            last_restock = datetime.now() - timedelta(days=random.randint(1, 90))
            inventory.append((pid, store_id, sup_id, stock, reorder, last_restock))
    execute_values(cur, f'''
        INSERT INTO "{schema}".inventory
        (product_id, store_id, supplier_id, stock_qty, reorder_level, last_restocked)
        VALUES %s''', inventory)

    # -------- customer_loyalty (one row per customer) --------
    loyalty = []
    for cid in customer_ids:
        spend = round(customer_spend.get(cid, 0.0), 2)
        if spend >= 3_000_000:
            tier = "platinum"
        elif spend >= 1_500_000:
            tier = "gold"
        elif spend >= 500_000:
            tier = "silver"
        else:
            tier = "bronze"
        points = int(spend // 1000)
        loyalty.append((
            cid, tier, points, spend,
            fake.date_between(start_date="-3y", end_date="-1m"),
        ))
    execute_values(cur, f'''
        INSERT INTO "{schema}".customer_loyalty
        (customer_id, tier, points, total_spend, member_since)
        VALUES %s''', loyalty)

    log.info("[%s] seeded: %d stores, %d employees, %d products, %d customers, "
             "%d transactions, %d items, %d inventory",
             schema, len(store_ids), len(emp_rows), len(product_ids),
             len(customer_ids), len(tx_rows), len(item_batch), len(inventory))


def main():
    log.info("connecting to postgres at %s:%s/%s", PG["host"], PG["port"], PG["dbname"])
    conn = psycopg2.connect(**PG)
    conn.autocommit = False
    try:
        with conn.cursor() as cur:
            for idx, schema in enumerate(TENANTS):
                seed_tenant(cur, schema.strip(), idx)
            conn.commit()
        log.info("seed completed successfully for tenants: %s", TENANTS)
    except Exception:
        conn.rollback()
        log.exception("seed failed, rolled back")
        raise
    finally:
        conn.close()


if __name__ == "__main__":
    main()
