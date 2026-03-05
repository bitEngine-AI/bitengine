import sqlite3
import os
from datetime import datetime
from flask import Flask, render_template, request, jsonify, g

app = Flask(__name__)
DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "crm.db")


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
        CREATE TABLE IF NOT EXISTS contacts (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            email TEXT DEFAULT '',
            phone TEXT DEFAULT '',
            company TEXT DEFAULT '',
            status TEXT NOT NULL DEFAULT 'lead',
            notes TEXT DEFAULT '',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )
    """)
    conn.commit()
    conn.close()


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/api/contacts", methods=["GET"])
def list_contacts():
    db = get_db()
    query = "SELECT * FROM contacts WHERE 1=1"
    params = []
    search = request.args.get("search", "").strip()
    status = request.args.get("status", "").strip()
    if search:
        query += " AND (name LIKE ? OR company LIKE ?)"
        params.extend([f"%{search}%", f"%{search}%"])
    if status:
        query += " AND status = ?"
        params.append(status)
    query += " ORDER BY created_at DESC"
    rows = db.execute(query, params).fetchall()
    return jsonify([dict(r) for r in rows])


@app.route("/api/contacts", methods=["POST"])
def create_contact():
    data = request.get_json()
    name = data.get("name", "").strip()
    if not name:
        return jsonify({"error": "name is required"}), 400
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    db = get_db()
    cur = db.execute(
        """INSERT INTO contacts (name, email, phone, company, status, notes, created_at, updated_at)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?)""",
        (name, data.get("email", ""), data.get("phone", ""),
         data.get("company", ""), data.get("status", "lead"),
         data.get("notes", ""), now, now),
    )
    db.commit()
    contact = dict(db.execute("SELECT * FROM contacts WHERE id = ?", (cur.lastrowid,)).fetchone())
    return jsonify(contact), 201


@app.route("/api/contacts/<int:cid>", methods=["GET"])
def get_contact(cid):
    db = get_db()
    row = db.execute("SELECT * FROM contacts WHERE id = ?", (cid,)).fetchone()
    if row is None:
        return jsonify({"error": "contact not found"}), 404
    return jsonify(dict(row))


@app.route("/api/contacts/<int:cid>", methods=["PUT"])
def update_contact(cid):
    data = request.get_json()
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    db = get_db()
    db.execute(
        """UPDATE contacts SET name=?, email=?, phone=?, company=?, status=?, notes=?, updated_at=?
           WHERE id=?""",
        (data.get("name", ""), data.get("email", ""), data.get("phone", ""),
         data.get("company", ""), data.get("status", "lead"),
         data.get("notes", ""), now, cid),
    )
    db.commit()
    row = db.execute("SELECT * FROM contacts WHERE id = ?", (cid,)).fetchone()
    if row is None:
        return jsonify({"error": "contact not found"}), 404
    return jsonify(dict(row))


@app.route("/api/contacts/<int:cid>", methods=["DELETE"])
def delete_contact(cid):
    db = get_db()
    db.execute("DELETE FROM contacts WHERE id = ?", (cid,))
    db.commit()
    return jsonify({"ok": True})


if __name__ == "__main__":
    init_db()
    app.run(host="0.0.0.0", port=5000, debug=False)
