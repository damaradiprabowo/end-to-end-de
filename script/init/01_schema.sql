-- =====================================================================
-- OLTP schema for the Minimarket POS system.
-- Three tenant schemas (tenant_01, tenant_02, tenant_03) are created,
-- each holding the full Beginner + Intermediate table set.
-- Every table carries created_at / updated_at to support incremental load.
-- =====================================================================

DO $$
DECLARE
    tenant TEXT;
    tenants TEXT[] := ARRAY['tenant_01', 'tenant_02', 'tenant_03'];
BEGIN
    FOREACH tenant IN ARRAY tenants
    LOOP
        EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I;', tenant);
        -- ---------------- customers ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.customers (
                customer_id   SERIAL PRIMARY KEY,
                name          VARCHAR(100) NOT NULL,
                phone         VARCHAR(20),
                email         VARCHAR(100),
                gender        VARCHAR(10),
                city          VARCHAR(50),
                created_at    TIMESTAMP DEFAULT NOW(),
                updated_at    TIMESTAMP DEFAULT NOW()
            );$f$, tenant);

        -- ---------------- products ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.products (
                product_id    SERIAL PRIMARY KEY,
                product_name  VARCHAR(150) NOT NULL,
                category      VARCHAR(50),
                brand         VARCHAR(50),
                unit_price    NUMERIC(12,2) NOT NULL,
                is_active     BOOLEAN DEFAULT TRUE,
                created_at    TIMESTAMP DEFAULT NOW(),
                updated_at    TIMESTAMP DEFAULT NOW()
            );$f$, tenant);

        -- ---------------- stores ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.stores (
                store_id      SERIAL PRIMARY KEY,
                store_name    VARCHAR(100) NOT NULL,
                city          VARCHAR(50),
                province      VARCHAR(50),
                store_type    VARCHAR(30),
                opened_at     DATE,
                is_active     BOOLEAN DEFAULT TRUE,
                created_at    TIMESTAMP DEFAULT NOW(),
                updated_at    TIMESTAMP DEFAULT NOW()
            );$f$, tenant);

        -- ---------------- suppliers ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.suppliers (
                supplier_id   SERIAL PRIMARY KEY,
                supplier_name VARCHAR(100),
                contact_name  VARCHAR(100),
                city          VARCHAR(50),
                country       VARCHAR(50) DEFAULT 'Indonesia',
                created_at    TIMESTAMP DEFAULT NOW(),
                updated_at    TIMESTAMP DEFAULT NOW()
            );$f$, tenant);

        -- ---------------- promotions ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.promotions (
                promo_id      SERIAL PRIMARY KEY,
                promo_name    VARCHAR(100),
                promo_type    VARCHAR(30),
                discount_pct  NUMERIC(5,2),
                start_date    DATE,
                end_date      DATE,
                min_purchase  NUMERIC(12,2) DEFAULT 0,
                created_at    TIMESTAMP DEFAULT NOW(),
                updated_at    TIMESTAMP DEFAULT NOW()
            );$f$, tenant);

        -- ---------------- employees (Advanced) ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.employees (
                employee_id   SERIAL PRIMARY KEY,
                store_id      INT REFERENCES %I.stores(store_id),
                name          VARCHAR(100),
                role          VARCHAR(50),
                hire_date     DATE,
                is_active     BOOLEAN DEFAULT TRUE,
                created_at    TIMESTAMP DEFAULT NOW(),
                updated_at    TIMESTAMP DEFAULT NOW()
            );$f$, tenant, tenant);

        -- ---------------- transactions ----------------
        -- employee_id (Advanced) included directly; FK added after employees.
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.transactions (
                transaction_id   SERIAL PRIMARY KEY,
                customer_id      INT REFERENCES %I.customers(customer_id),
                store_id         INT REFERENCES %I.stores(store_id),
                employee_id      INT REFERENCES %I.employees(employee_id),
                transaction_date TIMESTAMP NOT NULL,
                total_amount     NUMERIC(14,2) NOT NULL,
                payment_method   VARCHAR(30),
                status           VARCHAR(20) DEFAULT 'completed',
                created_at       TIMESTAMP DEFAULT NOW(),
                updated_at       TIMESTAMP DEFAULT NOW()
            );$f$, tenant, tenant, tenant, tenant);

        -- ---------------- transaction_items ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.transaction_items (
                item_id        SERIAL PRIMARY KEY,
                transaction_id INT REFERENCES %I.transactions(transaction_id),
                product_id     INT REFERENCES %I.products(product_id),
                quantity       INT NOT NULL,
                unit_price     NUMERIC(12,2) NOT NULL,
                discount       NUMERIC(5,2) DEFAULT 0,
                subtotal       NUMERIC(14,2) NOT NULL,
                created_at     TIMESTAMP DEFAULT NOW(),
                updated_at     TIMESTAMP DEFAULT NOW()
            );$f$, tenant, tenant, tenant);

        -- ---------------- transaction_promotions (junction) ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.transaction_promotions (
                id               SERIAL PRIMARY KEY,
                transaction_id   INT REFERENCES %I.transactions(transaction_id),
                promo_id         INT REFERENCES %I.promotions(promo_id),
                discount_applied NUMERIC(12,2),
                created_at       TIMESTAMP DEFAULT NOW(),
                updated_at       TIMESTAMP DEFAULT NOW()
            );$f$, tenant, tenant, tenant);

        -- ---------------- inventory (Advanced) ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.inventory (
                inventory_id   SERIAL PRIMARY KEY,
                product_id     INT REFERENCES %I.products(product_id),
                store_id       INT REFERENCES %I.stores(store_id),
                supplier_id    INT REFERENCES %I.suppliers(supplier_id),
                stock_qty      INT NOT NULL DEFAULT 0,
                reorder_level  INT DEFAULT 10,
                last_restocked TIMESTAMP,
                created_at     TIMESTAMP DEFAULT NOW(),
                updated_at     TIMESTAMP DEFAULT NOW()
            );$f$, tenant, tenant, tenant, tenant);

        -- ---------------- product_supplier (Advanced, many-to-many) ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.product_supplier (
                id             SERIAL PRIMARY KEY,
                product_id     INT REFERENCES %I.products(product_id),
                supplier_id    INT REFERENCES %I.suppliers(supplier_id),
                cost_price     NUMERIC(12,2),
                lead_time_days INT,
                is_primary     BOOLEAN DEFAULT FALSE,
                created_at     TIMESTAMP DEFAULT NOW(),
                updated_at     TIMESTAMP DEFAULT NOW()
            );$f$, tenant, tenant, tenant);

        -- ---------------- customer_loyalty (Advanced) ----------------
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I.customer_loyalty (
                loyalty_id     SERIAL PRIMARY KEY,
                customer_id    INT REFERENCES %I.customers(customer_id),
                tier           VARCHAR(20),
                points         INT DEFAULT 0,
                total_spend    NUMERIC(14,2) DEFAULT 0,
                member_since   DATE,
                created_at     TIMESTAMP DEFAULT NOW(),
                updated_at     TIMESTAMP DEFAULT NOW()
            );$f$, tenant, tenant);

        RAISE NOTICE 'Created schema and tables for %', tenant;
    END LOOP;
END $$;
