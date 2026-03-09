import json
import time

from garmin_data.database import Database


class TestSchema:
    def test_creates_health_data_table(self):
        db = Database(":memory:")
        tables = db.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()
        table_names = [t[0] for t in tables]
        assert "health_data" in table_names

    def test_creates_sync_log_table(self):
        db = Database(":memory:")
        tables = db.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()
        table_names = [t[0] for t in tables]
        assert "sync_log" in table_names


class TestUpsert:
    def test_insert_new_record(self):
        db = Database(":memory:")
        data = {"totalSteps": 8000}
        db.upsert("2026-03-09", "summary", data)

        row = db.query("2026-03-09", "summary")
        assert row is not None
        assert json.loads(row["data"]) == data

    def test_upsert_replaces_existing(self):
        db = Database(":memory:")
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        db.upsert("2026-03-09", "summary", {"totalSteps": 12000})

        row = db.query("2026-03-09", "summary")
        assert json.loads(row["data"])["totalSteps"] == 12000

    def test_upsert_updates_synced_at(self):
        db = Database(":memory:")
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        first_sync = db.query("2026-03-09", "summary")["synced_at"]

        time.sleep(0.01)
        db.upsert("2026-03-09", "summary", {"totalSteps": 12000})
        second_sync = db.query("2026-03-09", "summary")["synced_at"]

        assert second_sync > first_sync

    def test_different_metrics_same_date(self):
        db = Database(":memory:")
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        db.upsert("2026-03-09", "sleep", {"duration": 28800})

        summary = db.query("2026-03-09", "summary")
        sleep = db.query("2026-03-09", "sleep")
        assert json.loads(summary["data"])["totalSteps"] == 8000
        assert json.loads(sleep["data"])["duration"] == 28800


class TestQuery:
    def test_query_returns_none_for_missing(self):
        db = Database(":memory:")
        assert db.query("2026-03-09", "summary") is None

    def test_query_all_metrics_for_date(self):
        db = Database(":memory:")
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        db.upsert("2026-03-09", "sleep", {"duration": 28800})
        db.upsert("2026-03-10", "summary", {"totalSteps": 9000})

        rows = db.query_date("2026-03-09")
        assert len(rows) == 2
        metrics = {r["metric"] for r in rows}
        assert metrics == {"summary", "sleep"}


class TestSyncLog:
    def test_set_and_get(self):
        db = Database(":memory:")
        db.set_sync_log("last_sync", "2026-03-09")
        assert db.get_sync_log("last_sync") == "2026-03-09"

    def test_get_missing_returns_none(self):
        db = Database(":memory:")
        assert db.get_sync_log("last_sync") is None

    def test_set_overwrites(self):
        db = Database(":memory:")
        db.set_sync_log("last_sync", "2026-03-08")
        db.set_sync_log("last_sync", "2026-03-09")
        assert db.get_sync_log("last_sync") == "2026-03-09"


class TestRecordCount:
    def test_count_records(self):
        db = Database(":memory:")
        assert db.record_count() == 0
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        db.upsert("2026-03-09", "sleep", {"duration": 28800})
        assert db.record_count() == 2


class TestFilePath:
    def test_creates_db_file(self, tmp_path):
        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        assert db_path.exists()
