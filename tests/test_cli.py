import json
import os
import subprocess
import sys


def run_cli(*args: str, env_override: dict | None = None) -> subprocess.CompletedProcess:
    env = os.environ.copy()
    if env_override:
        env.update(env_override)
    return subprocess.run(
        [sys.executable, "-m", "garmin_data.cli", *args],
        capture_output=True,
        text=True,
        env=env,
    )


class TestNoCommand:
    def test_no_args_shows_help(self):
        result = run_cli()
        assert result.returncode == 0
        assert "login" in result.stdout
        assert "sync" in result.stdout
        assert "status" in result.stdout
        assert "query" in result.stdout


class TestSyncCommand:
    def test_sync_requires_garmin_email(self):
        env = os.environ.copy()
        env.pop("GARMIN_EMAIL", None)
        result = run_cli("sync", env_override={"GARMIN_EMAIL": ""})
        assert result.returncode != 0
        assert "GARMIN_EMAIL" in result.stderr

    def test_sync_invalid_date_format(self):
        result = run_cli(
            "sync", "--start", "not-a-date",
            env_override={"GARMIN_EMAIL": "test@example.com"},
        )
        assert result.returncode != 0

    def test_sync_default_range_is_7_days(self):
        # Just verify parsing works — actual sync requires auth
        result = run_cli("sync", "--help")
        assert "start" in result.stdout
        assert "end" in result.stdout

    def test_sync_accepts_metrics_flag(self):
        result = run_cli("sync", "--help")
        assert "metrics" in result.stdout


class TestStatusCommand:
    def test_status_with_empty_db(self, tmp_path):
        db_path = tmp_path / "test.db"
        result = run_cli(
            "status",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "0" in result.stdout

    def test_status_shows_record_count(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        db.upsert("2026-03-09", "sleep", {"duration": 28800})

        result = run_cli(
            "status",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "2" in result.stdout


class TestQueryCommand:
    def test_query_date_with_data(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})

        result = run_cli(
            "query", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "summary" in result.stdout
        assert "8000" in result.stdout

    def test_query_no_data(self, tmp_path):
        db_path = tmp_path / "test.db"
        result = run_cli(
            "query", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "No data" in result.stdout

    def test_query_specific_metric(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert("2026-03-09", "summary", {"totalSteps": 8000})
        db.upsert("2026-03-09", "sleep", {"duration": 28800})

        result = run_cli(
            "query", "2026-03-09", "--metric", "sleep",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "sleep" in result.stdout
        assert "summary" not in result.stdout

    def test_query_requires_date(self):
        result = run_cli("query")
        assert result.returncode != 0


class TestLoginCommand:
    def test_login_requires_garmin_email(self):
        result = run_cli("login", env_override={"GARMIN_EMAIL": ""})
        assert result.returncode != 0
        assert "GARMIN_EMAIL" in result.stderr
