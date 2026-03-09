import json
import sqlite3
import time


class Database:
    def __init__(self, path: str):
        self.conn = sqlite3.connect(path)
        self.conn.row_factory = sqlite3.Row
        self._create_tables()

    def _create_tables(self):
        self.conn.executescript("""
            CREATE TABLE IF NOT EXISTS health_data (
                date TEXT NOT NULL,
                metric TEXT NOT NULL,
                data TEXT NOT NULL,
                synced_at REAL NOT NULL,
                PRIMARY KEY (date, metric)
            );

            CREATE TABLE IF NOT EXISTS sync_log (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            );
        """)

    def execute(self, sql: str, params=()):
        return self.conn.execute(sql, params)

    def upsert(self, date: str, metric: str, data: dict):
        self.conn.execute(
            "INSERT OR REPLACE INTO health_data (date, metric, data, synced_at) VALUES (?, ?, ?, ?)",
            (date, metric, json.dumps(data), time.time()),
        )
        self.conn.commit()

    def query(self, date: str, metric: str) -> dict | None:
        row = self.conn.execute(
            "SELECT * FROM health_data WHERE date = ? AND metric = ?",
            (date, metric),
        ).fetchone()
        return dict(row) if row else None

    def query_date(self, date: str) -> list[dict]:
        rows = self.conn.execute(
            "SELECT * FROM health_data WHERE date = ? ORDER BY metric",
            (date,),
        ).fetchall()
        return [dict(r) for r in rows]

    def set_sync_log(self, key: str, value: str):
        self.conn.execute(
            "INSERT OR REPLACE INTO sync_log (key, value) VALUES (?, ?)",
            (key, value),
        )
        self.conn.commit()

    def get_sync_log(self, key: str) -> str | None:
        row = self.conn.execute(
            "SELECT value FROM sync_log WHERE key = ?",
            (key,),
        ).fetchone()
        return row["value"] if row else None

    def record_count(self) -> int:
        row = self.conn.execute("SELECT COUNT(*) as count FROM health_data").fetchone()
        return row["count"]
