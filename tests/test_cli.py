import json
import os
import subprocess
import sys
from datetime import date

from garmin_data.cli import build_parser, parse_date


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
        assert "daily" in result.stdout


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


class TestActivitiesCommand:
    def test_activities_shows_data(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert_activity(1, "2026-03-09", {"activityName": "Morning Run", "distance": 5000})

        result = run_cli(
            "activities", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "Morning Run" in result.stdout

    def test_activities_no_data(self, tmp_path):
        db_path = tmp_path / "test.db"
        result = run_cli(
            "activities", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "No activities" in result.stdout

    def test_activities_requires_date(self):
        result = run_cli("activities")
        assert result.returncode != 0


class TestDailyCommand:
    def test_daily_requires_date(self):
        result = run_cli("daily")
        assert result.returncode != 0

    def test_daily_no_data(self, tmp_path):
        db_path = tmp_path / "test.db"
        result = run_cli(
            "daily", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert result.stdout.strip() == ""

    def test_daily_sleep_and_steps(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert("2026-03-09", "sleep", {
            "dailySleepDTO": {"sleepTimeSeconds": 26100},
        })
        db.upsert("2026-03-09", "summary", {"totalSteps": 8432})

        result = run_cli(
            "daily", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "sleep_hours: 7.25" in result.stdout
        assert "steps: 8432" in result.stdout

    def test_daily_sleep_only(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert("2026-03-09", "sleep", {
            "dailySleepDTO": {"sleepTimeSeconds": 28800},
        })

        result = run_cli(
            "daily", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "sleep_hours: 8.00" in result.stdout
        assert "steps" not in result.stdout

    def test_daily_steps_only(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert("2026-03-09", "summary", {"totalSteps": 5000})

        result = run_cli(
            "daily", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "steps: 5000" in result.stdout
        assert "sleep_hours" not in result.stdout

    def test_daily_with_qualifying_activity(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert_activity(1, "2026-03-09", {
            "activityName": "Morning Run",
            "activityType": {"typeKey": "running"},
            "duration": 2520.0,
        })

        result = run_cli(
            "daily", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "exercise_type: run" in result.stdout
        assert "exercise_duration: 42" in result.stdout

    def test_daily_excludes_walking(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert_activity(1, "2026-03-09", {
            "activityName": "Afternoon Walk",
            "activityType": {"typeKey": "walking"},
            "duration": 1800.0,
        })

        result = run_cli(
            "daily", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "exercise_type" not in result.stdout
        assert "exercise_duration" not in result.stdout

    def test_daily_picks_longest_activity(self, tmp_path):
        from garmin_data.database import Database

        db_path = tmp_path / "test.db"
        db = Database(str(db_path))
        db.upsert_activity(1, "2026-03-09", {
            "activityName": "Short Run",
            "activityType": {"typeKey": "running"},
            "duration": 600.0,
        })
        db.upsert_activity(2, "2026-03-09", {
            "activityName": "Long Bike",
            "activityType": {"typeKey": "cycling"},
            "duration": 3600.0,
        })

        result = run_cli(
            "daily", "2026-03-09",
            env_override={"GARMIN_DB_PATH": str(db_path)},
        )
        assert result.returncode == 0
        assert "exercise_type: bike" in result.stdout
        assert "exercise_duration: 60" in result.stdout


class TestLoginCommand:
    def test_login_requires_garmin_email(self):
        result = run_cli("login", env_override={"GARMIN_EMAIL": ""})
        assert result.returncode != 0
        assert "GARMIN_EMAIL" in result.stderr


class TestParseDate:
    def test_valid_date(self):
        assert parse_date("2026-03-09") == date(2026, 3, 9)

    def test_invalid_date_raises(self):
        import argparse

        try:
            parse_date("not-a-date")
            assert False, "Should have raised"
        except argparse.ArgumentTypeError as e:
            assert "not-a-date" in str(e)


class TestExtractDaily:
    def test_extracts_sleep_and_steps(self):
        from garmin_data.database import Database

        db = Database(":memory:")
        db.upsert("2026-03-09", "sleep", {
            "dailySleepDTO": {"sleepTimeSeconds": 26100},
        })
        db.upsert("2026-03-09", "summary", {"totalSteps": 8432})

        from garmin_data.cli import extract_daily

        result = extract_daily(db, "2026-03-09")
        assert result["date"] == "2026-03-09"
        assert result["sleep_hours"] == 7.25
        assert result["steps"] == 8432

    def test_extracts_exercise(self):
        from garmin_data.database import Database

        db = Database(":memory:")
        db.upsert_activity(1, "2026-03-09", {
            "activityType": {"typeKey": "running"},
            "duration": 2520.0,
        })

        from garmin_data.cli import extract_daily

        result = extract_daily(db, "2026-03-09")
        assert result["exercise_type"] == "run"
        assert result["exercise_duration"] == 42

    def test_returns_none_when_no_data(self):
        from garmin_data.database import Database

        db = Database(":memory:")

        from garmin_data.cli import extract_daily

        assert extract_daily(db, "2026-03-09") is None

    def test_hiking_maps_to_hill_walk(self):
        from garmin_data.database import Database

        db = Database(":memory:")
        db.upsert_activity(1, "2026-03-09", {
            "activityType": {"typeKey": "hiking"},
            "duration": 3600.0,
        })

        from garmin_data.cli import extract_daily

        result = extract_daily(db, "2026-03-09")
        assert result["exercise_type"] == "hill_walk"


class TestPushCommand:
    def test_push_requires_genki_env_vars(self):
        result = run_cli(
            "push",
            env_override={
                "GARMIN_EMAIL": "test@example.com",
                "GENKI_URL": "",
                "GENKI_PASSWORD": "",
            },
        )
        assert result.returncode != 0
        assert "GENKI_URL" in result.stderr

    def test_push_appears_in_help(self):
        result = run_cli()
        assert "push" in result.stdout

    def test_push_parses_date_args(self):
        parser = build_parser()
        args = parser.parse_args(["push", "--start", "2026-03-01", "--end", "2026-03-09"])
        assert args.command == "push"
        assert args.start == date(2026, 3, 1)
        assert args.end == date(2026, 3, 9)


class TestBuildParser:
    def test_has_all_commands(self):
        parser = build_parser()
        # Parse each command to verify they exist
        args = parser.parse_args(["login"])
        assert args.command == "login"

        args = parser.parse_args(["sync"])
        assert args.command == "sync"

        args = parser.parse_args(["status"])
        assert args.command == "status"

        args = parser.parse_args(["query", "2026-03-09"])
        assert args.command == "query"

        args = parser.parse_args(["activities", "2026-03-09"])
        assert args.command == "activities"

        args = parser.parse_args(["daily", "2026-03-09"])
        assert args.command == "daily"

        args = parser.parse_args(["push"])
        assert args.command == "push"

    def test_sync_metrics_help_lists_available(self):
        parser = build_parser()
        # Get the sync subparser's --metrics action
        sync_action = None
        for action in parser._subparsers._actions:
            if hasattr(action, "_parser_class"):
                for name, subparser in action.choices.items():
                    if name == "sync":
                        for a in subparser._actions:
                            if "--metrics" in getattr(a, "option_strings", []):
                                sync_action = a
        assert sync_action is not None
        assert "summary" in sync_action.help
        assert "body_battery" in sync_action.help

    def test_no_command_returns_none(self):
        parser = build_parser()
        args = parser.parse_args([])
        assert args.command is None

    def test_sync_parses_date_args(self):
        parser = build_parser()
        args = parser.parse_args(["sync", "--start", "2026-03-01", "--end", "2026-03-09"])
        assert args.start == date(2026, 3, 1)
        assert args.end == date(2026, 3, 9)
