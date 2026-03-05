import sqlite3
import os
import random
from datetime import datetime, timedelta
from flask import Flask, render_template, jsonify, g

app = Flask(__name__)
DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "dashboard.db")


def get_db():
    if "db" not in g:
        g.db = sqlite3.connect(DB_PATH)
        g.db.row_factory = sqlite3.Row
    return g.db


@app.teardown_appcontext
def close_db(exception):
    db = g.pop("db", None)
    if db is not None:
        db.close()


def init_db():
    conn = sqlite3.connect(DB_PATH)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS daily_stats (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            date TEXT NOT NULL UNIQUE,
            sales REAL NOT NULL DEFAULT 0,
            visitors INTEGER NOT NULL DEFAULT 0,
            orders INTEGER NOT NULL DEFAULT 0,
            conversion_rate REAL NOT NULL DEFAULT 0
        )
    """)
    # Seed sample data if empty
    count = conn.execute("SELECT COUNT(*) FROM daily_stats").fetchone()[0]
    if count == 0:
        random.seed(42)
        base = datetime.now() - timedelta(days=29)
        for i in range(30):
            day = (base + timedelta(days=i)).strftime("%Y-%m-%d")
            visitors = random.randint(200, 1200)
            orders = random.randint(10, int(visitors * 0.15))
            sales = round(orders * random.uniform(50, 300), 2)
            rate = round(orders / visitors * 100, 2) if visitors > 0 else 0
            conn.execute(
                "INSERT INTO daily_stats (date, sales, visitors, orders, conversion_rate) VALUES (?, ?, ?, ?, ?)",
                (day, sales, visitors, orders, rate),
            )
        conn.commit()
    conn.close()


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/api/kpi", methods=["GET"])
def kpi():
    db = get_db()
    row = db.execute("""
        SELECT
            COALESCE(SUM(sales), 0) as total_sales,
            COALESCE(SUM(visitors), 0) as total_visitors,
            COALESCE(SUM(orders), 0) as total_orders,
            COALESCE(AVG(conversion_rate), 0) as avg_conversion
        FROM daily_stats
    """).fetchone()
    return jsonify({
        "total_sales": round(row["total_sales"], 2),
        "total_visitors": row["total_visitors"],
        "total_orders": row["total_orders"],
        "avg_conversion": round(row["avg_conversion"], 2),
    })


@app.route("/api/chart", methods=["GET"])
def chart_data():
    db = get_db()
    rows = db.execute("SELECT date, sales, visitors, orders FROM daily_stats ORDER BY date ASC").fetchall()
    data = [dict(r) for r in rows]
    return jsonify(data)


@app.route("/api/table", methods=["GET"])
def table_data():
    db = get_db()
    rows = db.execute("SELECT * FROM daily_stats ORDER BY date DESC").fetchall()
    return jsonify([dict(r) for r in rows])


if __name__ == "__main__":
    init_db()
    app.run(host="0.0.0.0", port=5000, debug=False)
