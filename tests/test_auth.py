import os
from unittest.mock import MagicMock, patch

from garminconnect import (
    GarminConnectAuthenticationError,
    GarminConnectConnectionError,
)

from garmin_data.auth import TOKEN_DIR, TOKEN_FILE, create_client, login


class TestCreateClient:
    def test_returns_garmin_instance(self):
        client = create_client("test@example.com")
        assert client.username == "test@example.com"

    def test_uses_email_parameter(self):
        client = create_client("lucy@example.com")
        assert client.username == "lucy@example.com"

    def test_sets_prompt_mfa(self):
        client = create_client("test@example.com")
        assert client.prompt_mfa is not None


class TestLogin:
    @patch("garmin_data.auth.create_client")
    def test_login_with_password(self, mock_create):
        mock_client = MagicMock()
        mock_client.client = MagicMock()
        mock_create.return_value = mock_client

        result = login("test@example.com", "secret123")

        mock_client.login.assert_called_once()
        mock_client.client.dump.assert_called_once_with(str(TOKEN_DIR))
        assert result is mock_client

    @patch("garmin_data.auth.create_client")
    def test_login_saves_tokens(self, mock_create):
        mock_client = MagicMock()
        mock_client.client = MagicMock()
        mock_create.return_value = mock_client

        login("test@example.com", "secret123")

        mock_client.client.dump.assert_called_once_with(str(TOKEN_DIR))

    @patch("garmin_data.auth.create_client")
    def test_login_propagates_auth_error(self, mock_create):
        mock_client = MagicMock()
        mock_client.login.side_effect = GarminConnectAuthenticationError("bad creds")
        mock_create.return_value = mock_client

        try:
            login("test@example.com", "wrong")
            assert False, "Expected GarminConnectAuthenticationError"
        except GarminConnectAuthenticationError:
            pass


class TestResumeSession:
    @patch("garmin_data.auth.TOKEN_FILE")
    @patch("garmin_data.auth.create_client")
    def test_resume_loads_tokens(self, mock_create, mock_token_file):
        mock_token_file.exists.return_value = True
        mock_client = MagicMock()
        mock_create.return_value = mock_client

        from garmin_data.auth import resume_session

        result = resume_session("test@example.com")

        mock_client.login.assert_called_once_with(tokenstore=str(TOKEN_DIR))
        assert result is mock_client

    @patch("garmin_data.auth.TOKEN_FILE")
    def test_resume_returns_none_when_no_tokens(self, mock_token_file):
        mock_token_file.exists.return_value = False

        from garmin_data.auth import resume_session

        result = resume_session("test@example.com")
        assert result is None
