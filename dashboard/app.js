// Unified dashboard: renders both the Intermediate questions (FastAPI) and the
// Advanced questions (Golang Data API) on a single page.
//
// API bases are overridable via query params:
//   ?int_api=http://host:8000   (FastAPI — Intermediate, brief §7)
//   ?adv_api=http://host:8090   (Go Data API — Advanced, brief §8, /api/v1/*)
const params = new URLSearchParams(location.search);
const INT_API = params.get("int_api") || "http://localhost:8000";
const ADV_API = params.get("adv_api") || "http://localhost:8090";

Chart.defaults.color = "#cbd5e1";
Chart.defaults.borderColor = "#334155";

const PALETTE = ["#38bdf8", "#f472b6", "#a78bfa", "#34d399", "#fbbf24",
                 "#fb7185", "#22d3ee", "#c084fc"];

async function getJSON(base, path) {
  const r = await fetch(`${base}${path}`);
  if (!r.ok) throw new Error(`${path} -> ${r.status}`);
  return r.json();
}

const groupBy = (rows, key) => rows.reduce((m, r) => {
  (m[r[key]] = m[r[key]] || []).push(r); return m;
}, {});

const c = id => document.getElementById(id).getContext("2d");

function opts(extra = {}) {
  const stacked = extra.stacked;
  return {
    responsive: true,
    indexAxis: extra.indexAxis || "x",
    plugins: { legend: { labels: { boxWidth: 12 } } },
    scales: { x: { stacked }, y: { stacked, beginAtZero: true } },
  };
}

function setStatus(msg, ok = true) {
  const el = document.getElementById("status");
  el.textContent = msg;
  el.style.color = ok ? "#94a3b8" : "#fb7185";
}

// ============================================================================
// Intermediate (FastAPI) — 5 questions
// ============================================================================

// IC1: revenue by store/month (multi-line)
async function ic1() {
  const rows = await getJSON(INT_API, "/api/revenue-by-store");
  const months = [...new Set(rows.map(r => r.year_month))].sort();
  const byStore = groupBy(rows, "store_name");
  const datasets = Object.entries(byStore).map(([store, rs], i) => {
    const map = Object.fromEntries(rs.map(r => [r.year_month, r.revenue]));
    return { label: store, data: months.map(m => map[m] ?? 0),
             borderColor: PALETTE[i % PALETTE.length], tension: .3, fill: false };
  });
  new Chart(c("ic1"), { type: "line", data: { labels: months, datasets }, options: opts() });
}

// IC2: promotion effectiveness (horizontal bar). Endpoint returns {promotions, avg_transaction}.
async function ic2() {
  const data = await getJSON(INT_API, "/api/promotion-effectiveness");
  const rows = data.promotions.slice(0, 10);
  new Chart(c("ic2"), {
    type: "bar",
    data: { labels: rows.map(r => r.promo_name),
            datasets: [{ label: "Total discount", data: rows.map(r => r.total_discount),
                         backgroundColor: PALETTE[1] }] },
    options: opts({ indexAxis: "y" }),
  });
}

// IC3: top products by city (grouped horizontal bar)
async function ic3() {
  const rows = await getJSON(INT_API, "/api/top-products-by-city");
  const cities = [...new Set(rows.map(r => r.city))];
  const datasets = [0, 1, 2].map(rank => ({
    label: `#${rank + 1}`,
    backgroundColor: PALETTE[rank],
    data: cities.map(city => {
      const list = rows.filter(r => r.city === city);
      return list[rank] ? list[rank].revenue : 0;
    }),
  }));
  new Chart(c("ic3"), { type: "bar", data: { labels: cities, datasets }, options: opts({ indexAxis: "y" }) });
}

// IC4: customer segments per city (stacked bar)
async function ic4() {
  const rows = await getJSON(INT_API, "/api/customer-segments");
  const cities = [...new Set(rows.map(r => r.city))];
  const segs = ["High", "Medium", "Low"];
  const datasets = segs.map((seg, i) => ({
    label: seg, backgroundColor: PALETTE[i],
    data: cities.map(city => {
      const r = rows.find(x => x.city === city && x.segment === seg);
      return r ? r.customers : 0;
    }),
  }));
  new Chart(c("ic4"), { type: "bar", data: { labels: cities, datasets }, options: opts({ stacked: true }) });
}

// IC5: transactions by day of week (bar + line)
async function ic5() {
  const rows = await getJSON(INT_API, "/api/transactions-by-day");
  new Chart(c("ic5"), {
    data: {
      labels: rows.map(r => r.day_name),
      datasets: [
        { type: "bar", label: "Transactions", data: rows.map(r => r.transactions),
          backgroundColor: PALETTE[0], yAxisID: "y" },
        { type: "line", label: "Revenue", data: rows.map(r => r.revenue),
          borderColor: PALETTE[4], tension: .3, yAxisID: "y1" },
      ],
    },
    options: { responsive: true, scales: {
      y: { position: "left" }, y1: { position: "right", grid: { drawOnChartArea: false } } } },
  });
}

// ============================================================================
// Advanced (Golang Data API, /api/v1/*) — 7 questions
// ============================================================================

// AC1: revenue & margin per category per quarter (revenue stacked by category)
async function ac1() {
  const rows = await getJSON(ADV_API, "/api/v1/revenue-margin");
  const quarters = [...new Set(rows.map(r => r.quarter))].sort();
  const cats = [...new Set(rows.map(r => r.category))];
  const datasets = cats.map((cat, i) => ({
    label: cat,
    backgroundColor: PALETTE[i % PALETTE.length],
    data: quarters.map(q => {
      const r = rows.find(x => x.quarter === q && x.category === cat);
      return r ? r.revenue : 0;
    }),
  }));
  new Chart(c("ac1"), { type: "bar", data: { labels: quarters, datasets }, options: opts({ stacked: true }) });
}

// AC2: customer lifetime value, top 20 (horizontal bar, labelled with tier)
async function ac2() {
  const rows = await getJSON(ADV_API, "/api/v1/customer-ltv");
  new Chart(c("ac2"), {
    type: "bar",
    data: { labels: rows.map(r => `${r.customer_name} · ${r.tier}`),
            datasets: [{ label: "Lifetime value", data: rows.map(r => r.lifetime_value),
                         backgroundColor: PALETTE[2] }] },
    options: opts({ indexAxis: "y" }),
  });
}

// AC3: inventory turnover, slowest movers (horizontal bar)
async function ac3() {
  const rows = await getJSON(ADV_API, "/api/v1/inventory-turnover");
  new Chart(c("ac3"), {
    type: "bar",
    data: { labels: rows.map(r => `${r.product_name} · ${r.store_name}`),
            datasets: [{ label: "Turnover rate (qty_sold / stock)", data: rows.map(r => r.turnover_rate),
                         backgroundColor: PALETTE[5] }] },
    options: opts({ indexAxis: "y" }),
  });
}

// AC4: employee effectiveness — avg transaction value per cashier (horizontal bar)
async function ac4() {
  const rows = await getJSON(ADV_API, "/api/v1/employee-performance");
  new Chart(c("ac4"), {
    type: "bar",
    data: {
      labels: rows.map(r => `${r.employee_name} · ${r.store_city}`),
      datasets: [
        { label: "Avg txn value", data: rows.map(r => r.avg_txn_value), backgroundColor: PALETTE[0] },
        { label: "Revenue", data: rows.map(r => r.revenue), backgroundColor: PALETTE[3], hidden: true },
      ],
    },
    options: opts({ indexAxis: "y" }),
  });
}

// AC5: promotion ROI (horizontal bar, green positive / red negative)
async function ac5() {
  const rows = await getJSON(ADV_API, "/api/v1/promotion-roi");
  new Chart(c("ac5"), {
    type: "bar",
    data: { labels: rows.map(r => r.promo_name),
            datasets: [{ label: "ROI (incremental revenue / discount)", data: rows.map(r => r.roi),
                         backgroundColor: rows.map(r => (r.roi >= 0 ? PALETTE[3] : PALETTE[5])) }] },
    options: opts({ indexAxis: "y" }),
  });
}

// AC6: supplier dependency for top sellers (revenue bar, colour = sourcing)
async function ac6() {
  const rows = await getJSON(ADV_API, "/api/v1/supplier-dependency");
  new Chart(c("ac6"), {
    type: "bar",
    data: {
      labels: rows.map(r => r.product_name),
      datasets: [{
        label: "Revenue — red = single-source, green = multi-source",
        data: rows.map(r => r.revenue),
        backgroundColor: rows.map(r => (r.sourcing === "single-source" ? PALETTE[5] : PALETTE[3])),
      }],
    },
    options: opts({ indexAxis: "y" }),
  });
}

// AC7: customer retention cohort (heatmap via matrix controller)
async function ac7() {
  const rows = await getJSON(ADV_API, "/api/v1/cohort-retention");
  const cohorts = [...new Set(rows.map(r => r.cohort_month))].sort();          // y axis
  const offsets = [...new Set(rows.map(r => r.month_offset))].sort((a, b) => a - b); // x axis

  // color ramp: low retention -> dark slate, high retention -> teal
  const color = pct => {
    const t = Math.max(0, Math.min(1, pct / 100));
    const r = Math.round(30 + t * (34 - 30));
    const g = Math.round(41 + t * (211 - 41));
    const b = Math.round(59 + t * (153 - 59));
    return `rgba(${r},${g},${b},${0.25 + 0.75 * t})`;
  };

  const xLabels = offsets.map(o => `+${o}`);            // category labels, centered per cell
  const data = rows.map(r => ({
    x: `+${r.month_offset}`, y: r.cohort_month, v: r.retention_pct, size: r.cohort_size,
  }));

  new Chart(c("ac7"), {
    type: "matrix",
    data: {
      datasets: [{
        label: "Retention %",
        data,
        backgroundColor: ctx => color(ctx.raw.v),
        borderWidth: 1,
        borderColor: "#0f172a",
        width: ({ chart }) => (chart.chartArea?.width ?? 0) / offsets.length - 2,
        height: ({ chart }) => (chart.chartArea?.height ?? 0) / cohorts.length - 2,
      }],
    },
    options: {
      responsive: true,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            title: items => `Cohort ${items[0].raw.y}`,
            label: ctx => `${ctx.raw.x} mo: ${ctx.raw.v}% (${ctx.raw.size} customers)`,
          },
        },
      },
      // both axes are category scales so labels sit centered under/beside each cell
      scales: {
        x: { type: "category", labels: xLabels, position: "top", offset: true,
             grid: { display: false }, title: { display: true, text: "Months since first purchase" } },
        y: { type: "category", labels: [...cohorts].reverse(), offset: true,
             grid: { display: false }, title: { display: true, text: "Cohort month" } },
      },
    },
  });
}

// ============================================================================
// boot — load each section independently so one API being down doesn't blank the other
// ============================================================================
(async () => {
  const status = [];
  try {
    await Promise.all([ic1(), ic2(), ic3(), ic4(), ic5()]);
    status.push(`Intermediate API ✓ (${INT_API})`);
  } catch (e) {
    status.push(`Intermediate API ✗ (${e.message})`);
  }
  try {
    await Promise.all([ac1(), ac2(), ac3(), ac4(), ac5(), ac6(), ac7()]);
    status.push(`Advanced API ✓ (${ADV_API})`);
  } catch (e) {
    status.push(`Advanced API ✗ (${e.message})`);
  }
  setStatus(status.join("   |   "), !status.some(s => s.includes("✗")));
})();
