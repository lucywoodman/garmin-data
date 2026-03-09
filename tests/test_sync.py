import json
from datetime import date
from unittest.mock import MagicMock, call, patch

from garmin_data.database import Database
from garmin_data.sync import METRICS, sync_metrics


class TestMetricsConfig:
    def test_all_four_metrics_defined(self):
        assert set(METRICS.keys()) == {"summary", "heart_rate", "rhr", "sleep"}

    def test_each_metric_has_api_method(self):
        for name, method_name in METRICS.items():
            assert isinstance(method_name, str)


class TestSyncMetrics:
    def _make_client(self):
        client = MagicMock()
        client.get_user_summary.return_value = {"totalSteps": 8000}
        client.get_heart_rates.return_value = {"heartRateValues": [70, 80]}
        client.get_rhr_day.return_value = {"restingHeartRate": 55}
        client.get_sleep_data.return_value = {"sleepDuration": 28800}
        return client

    @patch("garmin_data.sync.time.sleep")
    def test_syncs_all_metrics_for_single_day(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        sync_metrics(client, db, date(2026, 3, 9), date(2026, 3, 9))

        assert db.record_count() == 4
        summary = db.query("2026-03-09", "summary")
        assert json.loads(summary["data"])["totalSteps"] == 8000

    @patch("garmin_data.sync.time.sleep")
    def test_syncs_date_range(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        sync_metrics(client, db, date(2026, 3, 8), date(2026, 3, 9))

        # 2 days * 4 metrics = 8 records
        assert db.record_count() == 8

    @patch("garmin_data.sync.time.sleep")
    def test_syncs_specific_metrics(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        sync_metrics(
            client, db, date(2026, 3, 9), date(2026, 3, 9),
            metric_names=["summary", "sleep"],
        )

        assert db.record_count() == 2
        assert db.query("2026-03-09", "summary") is not None
        assert db.query("2026-03-09", "sleep") is not None
        assert db.query("2026-03-09", "heart_rate") is None

    @patch("garmin_data.sync.time.sleep")
    def test_idempotent_resync(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        sync_metrics(client, db, date(2026, 3, 9), date(2026, 3, 9))
        sync_metrics(client, db, date(2026, 3, 9), date(2026, 3, 9))

        # Still 4 records, not 8
        assert db.record_count() == 4

    @patch("garmin_data.sync.time.sleep")
    def test_rate_limit_delay_between_calls(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        sync_metrics(client, db, date(2026, 3, 9), date(2026, 3, 9))

        # 4 metrics, delay between each (3 sleeps for 4 calls)
        assert mock_sleep.call_count == 3
        mock_sleep.assert_called_with(0.5)

    @patch("garmin_data.sync.time.sleep")
    def test_updates_sync_log(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        sync_metrics(client, db, date(2026, 3, 8), date(2026, 3, 9))

        assert db.get_sync_log("last_sync_date") == "2026-03-09"

    @patch("garmin_data.sync.time.sleep")
    def test_429_triggers_backoff(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        from garminconnect import GarminConnectTooManyRequestsError

        call_count = 0
        original_return = {"totalSteps": 8000}

        def flaky_summary(cdate):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                raise GarminConnectTooManyRequestsError("429")
            return original_return

        client.get_user_summary.side_effect = flaky_summary

        sync_metrics(
            client, db, date(2026, 3, 9), date(2026, 3, 9),
            metric_names=["summary"],
        )

        # Should have retried after backoff
        assert db.query("2026-03-09", "summary") is not None
        # Check 60s backoff was called
        assert call(60) in mock_sleep.call_args_list

    @patch("garmin_data.sync.time.sleep")
    def test_calls_correct_api_methods(self, mock_sleep):
        db = Database(":memory:")
        client = self._make_client()

        sync_metrics(client, db, date(2026, 3, 9), date(2026, 3, 9))

        client.get_user_summary.assert_called_with("2026-03-09")
        client.get_heart_rates.assert_called_with("2026-03-09")
        client.get_rhr_day.assert_called_with("2026-03-09")
        client.get_sleep_data.assert_called_with("2026-03-09")
