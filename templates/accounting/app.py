import sqlite3
import os
from datetime import datetime
from flask import Flask, render_template, request, jsonify, g

app = Flask(__name__)
DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "accounting.db")

INCOME_CATEGORIES = ["salary", "freelance", "investment", "gift", "other_income"]
EXPENSE_CATEGORIES = ["food", "transport", "housing", "entertainment", "shopping", "health", "education", "other_expense"]
CATEGORY_LABELS = {
    "salary": "工资", "freelance": "兼职", "investment": "投资收益",
    "gift": "礼金", "other_income": "其他收入",
    "food": "餐饮", "transport": "交通", "housing": "住房",
    "entertainment": "娱乐", "shopping": "购物", "health": "医疗",
    "education": "教育", "other_expense": "其他支出",
}


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
        CREATE TABLE IF NOT EXISTS transactions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            date TEXT NOT NULL,
            amount REAL NOT NULL,
            type TEXT NOT NULL,
            category TEXT NOT NULL,
            note TEXT DEFAULT '',
            created_at TEXT NOT NULL
        )
    """)
    conn.commit()
    conn.close()


@app.route("/")
def index():
    return render_template("index.html", category_labels=CATEGORY_LABELS,
                           income_categories=INCOME_CATEGORIES,
                           expense_categories=EXPENSE_CATEGORIES)


@app.route("/api/transactions", methods=["GET"])
def list_transactions():
    db = get_db()
    query = "SELECT * FROM transactions WHERE 1=1"
    params = []
    date_from = request.args.get("date_from")
    date_to = request.args.get("date_to")
    category = request.args.get("category")
    if date_from:
        query += " AND date >= ?"
        params.append(date_from)
    if date_to:
        query += " AND date <= ?"
        params.append(date_to)
    if category:
        query += " AND category = ?"
        params.append(category)
    query += " ORDER BY date DESC, id DESC"
    rows = db.execute(query, params).fetchall()
    transactions = [dict(r) for r in rows]
    return jsonify(transactions)


@app.route("/api/transactions", methods=["POST"])
def create_transaction():
    data = request.get_json()
    date_val = data.get("date", datetime.now().strftime("%Y-%m-%d"))
    amount = data.get("amount")
    category = data.get("category", "")
    note = data.get("note", "")
    if not amount or float(amount) <= 0:
        return jsonify({"error": "amount must be positive"}), 400
    tx_type = "income" if category in INCOME_CATEGORIES else "expense"
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    db = get_db()
    cur = db.execute(
        "INSERT INTO transactions (date, amount, type, category, note, created_at) VALUES (?, ?, ?, ?, ?, ?)",
        (date_val, float(amount), tx_type, category, note, now),
    )
    db.commit()
    tx = dict(db.execute("SELECT * FROM transactions WHERE id = ?", (cur.lastrowid,)).fetchone())
    return jsonify(tx), 201


@app.route("/api/transactions/<int:tx_id>", methods=["DELETE"])
def delete_transaction(tx_id):
    db = get_db()
    db.execute("DELETE FROM transactions WHERE id = ?", (tx_id,))
    db.commit()
    return jsonify({"ok": True})


@app.route("/api/summary", methods=["GET"])
def summary():
    db = get_db()
    date_from = request.args.get("date_from")
    date_to = request.args.get("date_to")
    query_income = "SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type='income'"
    query_expense = "SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type='expense'"
    params_i, params_e = [], []
    if date_from:
        query_income += " AND date >= ?"
        query_expense += " AND date >= ?"
        params_i.append(date_from)
        params_e.append(date_from)
    if date_to:
        query_income += " AND date <= ?"
        query_expense += " AND date <= ?"
        params_i.append(date_to)
        params_e.append(date_to)
    income = db.execute(query_income, params_i).fetchone()[0]
    expense = db.execute(query_expense, params_e).fetchone()[0]
    return jsonify({"income": income, "expense": expense, "balance": income - expense})


if __name__ == "__main__":
    init_db()
    app.run(host="0.0.0.0", port=5000, debug=False)
