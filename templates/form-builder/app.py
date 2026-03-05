import sqlite3
import os
import json
from datetime import datetime
from flask import Flask, render_template, request, jsonify, g

app = Flask(__name__)
DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "formbuilder.db")


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
        CREATE TABLE IF NOT EXISTS forms (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            description TEXT DEFAULT '',
            fields TEXT NOT NULL DEFAULT '[]',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )
    """)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS submissions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            form_id INTEGER NOT NULL,
            data TEXT NOT NULL DEFAULT '{}',
            submitted_at TEXT NOT NULL,
            FOREIGN KEY (form_id) REFERENCES forms(id) ON DELETE CASCADE
        )
    """)
    conn.commit()
    conn.close()


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/api/forms", methods=["GET"])
def list_forms():
    db = get_db()
    rows = db.execute("SELECT * FROM forms ORDER BY created_at DESC").fetchall()
    forms = []
    for r in rows:
        f = dict(r)
        f["fields"] = json.loads(f["fields"])
        sub_count = db.execute("SELECT COUNT(*) FROM submissions WHERE form_id = ?", (f["id"],)).fetchone()[0]
        f["submission_count"] = sub_count
        forms.append(f)
    return jsonify(forms)


@app.route("/api/forms", methods=["POST"])
def create_form():
    data = request.get_json()
    title = data.get("title", "").strip()
    if not title:
        return jsonify({"error": "title is required"}), 400
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    fields = json.dumps(data.get("fields", []), ensure_ascii=False)
    db = get_db()
    cur = db.execute(
        "INSERT INTO forms (title, description, fields, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
        (title, data.get("description", ""), fields, now, now),
    )
    db.commit()
    row = dict(db.execute("SELECT * FROM forms WHERE id = ?", (cur.lastrowid,)).fetchone())
    row["fields"] = json.loads(row["fields"])
    return jsonify(row), 201


@app.route("/api/forms/<int:fid>", methods=["GET"])
def get_form(fid):
    db = get_db()
    row = db.execute("SELECT * FROM forms WHERE id = ?", (fid,)).fetchone()
    if row is None:
        return jsonify({"error": "form not found"}), 404
    f = dict(row)
    f["fields"] = json.loads(f["fields"])
    return jsonify(f)


@app.route("/api/forms/<int:fid>", methods=["PUT"])
def update_form(fid):
    data = request.get_json()
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    fields = json.dumps(data.get("fields", []), ensure_ascii=False)
    db = get_db()
    db.execute(
        "UPDATE forms SET title=?, description=?, fields=?, updated_at=? WHERE id=?",
        (data.get("title", ""), data.get("description", ""), fields, now, fid),
    )
    db.commit()
    row = db.execute("SELECT * FROM forms WHERE id = ?", (fid,)).fetchone()
    if row is None:
        return jsonify({"error": "form not found"}), 404
    f = dict(row)
    f["fields"] = json.loads(f["fields"])
    return jsonify(f)


@app.route("/api/forms/<int:fid>", methods=["DELETE"])
def delete_form(fid):
    db = get_db()
    db.execute("DELETE FROM submissions WHERE form_id = ?", (fid,))
    db.execute("DELETE FROM forms WHERE id = ?", (fid,))
    db.commit()
    return jsonify({"ok": True})


@app.route("/api/forms/<int:fid>/submit", methods=["POST"])
def submit_form(fid):
    db = get_db()
    form = db.execute("SELECT * FROM forms WHERE id = ?", (fid,)).fetchone()
    if form is None:
        return jsonify({"error": "form not found"}), 404
    data = request.get_json()
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    db.execute(
        "INSERT INTO submissions (form_id, data, submitted_at) VALUES (?, ?, ?)",
        (fid, json.dumps(data, ensure_ascii=False), now),
    )
    db.commit()
    return jsonify({"ok": True}), 201


@app.route("/api/forms/<int:fid>/submissions", methods=["GET"])
def list_submissions(fid):
    db = get_db()
    rows = db.execute("SELECT * FROM submissions WHERE form_id = ? ORDER BY submitted_at DESC", (fid,)).fetchall()
    subs = []
    for r in rows:
        s = dict(r)
        s["data"] = json.loads(s["data"])
        subs.append(s)
    return jsonify(subs)


if __name__ == "__main__":
    init_db()
    app.run(host="0.0.0.0", port=5000, debug=False)
