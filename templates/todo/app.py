import sqlite3
import os
from datetime import datetime
from flask import Flask, render_template, request, jsonify, g

app = Flask(__name__)
DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "todo.db")


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
        CREATE TABLE IF NOT EXISTS tasks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            status TEXT NOT NULL DEFAULT 'todo',
            priority TEXT NOT NULL DEFAULT 'medium',
            created_at TEXT NOT NULL
        )
    """)
    conn.commit()
    conn.close()


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/api/tasks", methods=["GET"])
def list_tasks():
    db = get_db()
    rows = db.execute("SELECT * FROM tasks ORDER BY created_at DESC").fetchall()
    tasks = [dict(r) for r in rows]
    return jsonify(tasks)


@app.route("/api/tasks", methods=["POST"])
def create_task():
    data = request.get_json()
    title = data.get("title", "").strip()
    if not title:
        return jsonify({"error": "title is required"}), 400
    priority = data.get("priority", "medium")
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    db = get_db()
    cur = db.execute(
        "INSERT INTO tasks (title, status, priority, created_at) VALUES (?, ?, ?, ?)",
        (title, "todo", priority, now),
    )
    db.commit()
    task = dict(db.execute("SELECT * FROM tasks WHERE id = ?", (cur.lastrowid,)).fetchone())
    return jsonify(task), 201


@app.route("/api/tasks/<int:task_id>/status", methods=["PUT"])
def update_status(task_id):
    data = request.get_json()
    status = data.get("status")
    if status not in ("todo", "in_progress", "done"):
        return jsonify({"error": "invalid status"}), 400
    db = get_db()
    db.execute("UPDATE tasks SET status = ? WHERE id = ?", (status, task_id))
    db.commit()
    task = db.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
    if task is None:
        return jsonify({"error": "task not found"}), 404
    return jsonify(dict(task))


@app.route("/api/tasks/<int:task_id>", methods=["DELETE"])
def delete_task(task_id):
    db = get_db()
    db.execute("DELETE FROM tasks WHERE id = ?", (task_id,))
    db.commit()
    return jsonify({"ok": True})


if __name__ == "__main__":
    init_db()
    app.run(host="0.0.0.0", port=5000, debug=False)
