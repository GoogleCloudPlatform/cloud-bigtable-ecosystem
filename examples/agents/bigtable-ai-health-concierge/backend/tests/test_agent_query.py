import importlib
import os
import sys
import unittest
from datetime import datetime, timezone
from unittest.mock import AsyncMock, MagicMock, patch

from google.adk.tools.bigtable.bigtable_toolset import BigtableToolset
from google.adk.tools.google_tool import GoogleTool
from google.adk.tools.tool_context import ToolContext
from google.auth.credentials import Credentials
from pydantic import ValidationError

BACKEND_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
if BACKEND_DIR not in sys.path:
    sys.path.insert(0, BACKEND_DIR)


def _load_agent_query():
    """Imports sub_agents.agent_query with mocked GCP ADC credentials."""
    if "sub_agents.agent_query" in sys.modules:
        return sys.modules["sub_agents.agent_query"]
    mock_creds = MagicMock(spec=Credentials)
    with patch("google.auth.default", return_value=(mock_creds, "test-project")):
        return importlib.import_module("sub_agents.agent_query")


agent_query = _load_agent_query()


class TestAgentQuerySecurity(unittest.IsolatedAsyncioTestCase):
    """Security and regression tests for MedicalRecordsAgent Bigtable tools."""

    def setUp(self):
        self.mock_creds = MagicMock(spec=Credentials)
        self.mock_settings = agent_query.tool_settings
        self.mock_context = MagicMock(spec=ToolContext)

    def test_agent_query_does_not_expose_raw_bigtable_toolset(self):
        """Agent_Query must only expose scoped GoogleTool wrappers."""
        self.assertGreater(len(agent_query.Agent_Query.tools), 0)
        exposed_names = set()
        for tool in agent_query.Agent_Query.tools:
            self.assertNotIsInstance(tool, BigtableToolset)
            self.assertIsInstance(tool, GoogleTool)
            decl = tool._get_declaration()
            exposed_names.add(decl.name)

        self.assertEqual(
            exposed_names,
            {
                "get_test_results",
                "get_prescriptions",
                "get_visits",
                "get_test_results_specific_test",
            },
        )
        self.assertFalse(hasattr(agent_query, "bigtable_toolset"))

    def test_helper_guards_reject_invalid_inputs(self):
        """In-body helpers reject invalid types/ranges even if called directly."""
        for bad_ts in ["2025-01-01", 12345, None]:
            with self.subTest(bad_ts=bad_ts):
                with self.assertRaises(ValueError):
                    agent_query._format_sql_timestamp(bad_ts)

        for bad_num in [True, False, "10", 0, -1, 301]:
            with self.subTest(bad_num=bad_num):
                with self.assertRaises(ValueError):
                    agent_query._validate_num_tests(bad_num)

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_get_test_results_parameterizes_patient_key(
        self, mock_execute_sql
    ):
        mock_execute_sql.return_value = {"status": "SUCCESS", "rows": []}
        before = datetime(2026, 2, 1, 0, 0, 0, tzinfo=timezone.utc)
        after = datetime(2025, 1, 1, 0, 0, 0, tzinfo=timezone.utc)

        res = await agent_query.get_test_results(
            self.mock_creds,
            self.mock_settings,
            self.mock_context,
            before=before,
            after=after,
            num_tests=10,
        )

        self.assertEqual(res["status"], "SUCCESS")
        mock_execute_sql.assert_awaited_once()
        kwargs = mock_execute_sql.call_args.kwargs
        self.assertIn("WHERE _key = @patient_key", kwargs["query"])
        self.assertNotIn("john.doe@gmail.com", kwargs["query"])
        self.assertIn(
            "before=>TIMESTAMP('2026-02-01T00:00:00+00:00')", kwargs["query"]
        )
        self.assertIn(
            "after=>TIMESTAMP('2025-01-01T00:00:00+00:00')", kwargs["query"]
        )
        self.assertIn("latest_n=>10", kwargs["query"])
        self.assertEqual(
            kwargs["parameters"], {"patient_key": b"john.doe@gmail.com"}
        )

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_iso_string_timestamps_from_llm_tool_call_accepted(
        self, mock_execute_sql
    ):
        """ISO 8601 strings passed from Gemini/ADK JSON args are coerced cleanly."""
        mock_execute_sql.return_value = {"status": "SUCCESS", "rows": []}

        res = await agent_query.get_test_results(
            self.mock_creds,
            self.mock_settings,
            self.mock_context,
            before="2026-02-01T00:00:00Z",
            after="2025-01-01T00:00:00+00:00",
            num_tests=5,
        )

        self.assertEqual(res["status"], "SUCCESS")
        kwargs = mock_execute_sql.call_args.kwargs
        self.assertIn(
            "before=>TIMESTAMP('2026-02-01T00:00:00+00:00')", kwargs["query"]
        )
        self.assertIn(
            "after=>TIMESTAMP('2025-01-01T00:00:00+00:00')", kwargs["query"]
        )

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_get_prescriptions_parameterizes_patient_key(
        self, mock_execute_sql
    ):
        mock_execute_sql.return_value = {"status": "SUCCESS", "rows": []}

        res = await agent_query.get_prescriptions(
            self.mock_creds, self.mock_settings, self.mock_context
        )

        self.assertEqual(res["status"], "SUCCESS")
        mock_execute_sql.assert_awaited_once()
        kwargs = mock_execute_sql.call_args.kwargs
        self.assertIn("WHERE _key = @patient_key", kwargs["query"])
        self.assertNotIn("john.doe@gmail.com", kwargs["query"])
        self.assertEqual(
            kwargs["parameters"], {"patient_key": b"john.doe@gmail.com"}
        )

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_get_visits_parameterizes_patient_key(
        self, mock_execute_sql
    ):
        mock_execute_sql.return_value = {"status": "SUCCESS", "rows": []}

        res = await agent_query.get_visits(
            self.mock_creds, self.mock_settings, self.mock_context
        )

        self.assertEqual(res["status"], "SUCCESS")
        mock_execute_sql.assert_awaited_once()
        kwargs = mock_execute_sql.call_args.kwargs
        self.assertIn("WHERE _key = @patient_key", kwargs["query"])
        self.assertNotIn("john.doe@gmail.com", kwargs["query"])
        self.assertEqual(
            kwargs["parameters"], {"patient_key": b"john.doe@gmail.com"}
        )

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_get_test_results_specific_test_all_and_specific(
        self, mock_execute_sql
    ):
        mock_execute_sql.return_value = {"status": "SUCCESS", "rows": []}

        await agent_query.get_test_results_specific_test(
            self.mock_creds,
            self.mock_settings,
            self.mock_context,
            test_name="all",
            num_tests=5,
        )
        all_kwargs = mock_execute_sql.call_args.kwargs
        self.assertIn("SELECT tests as results", all_kwargs["query"])
        self.assertEqual(
            all_kwargs["parameters"], {"patient_key": b"john.doe@gmail.com"}
        )

        mock_execute_sql.reset_mock()

        await agent_query.get_test_results_specific_test(
            self.mock_creds,
            self.mock_settings,
            self.mock_context,
            test_name="Glucose",
            num_tests=5,
        )
        glucose_kwargs = mock_execute_sql.call_args.kwargs
        self.assertIn(
            "SELECT tests[@test_name] as results", glucose_kwargs["query"]
        )
        self.assertNotIn("Glucose", glucose_kwargs["query"])
        self.assertEqual(
            glucose_kwargs["parameters"],
            {"patient_key": b"john.doe@gmail.com", "test_name": b"Glucose"},
        )

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_sql_errors_are_sanitized(self, mock_execute_sql):
        mock_execute_sql.return_value = {
            "status": "ERROR",
            "error_details": "400 Internal Bigtable SQL syntax details",
        }

        res = await agent_query.get_prescriptions(
            self.mock_creds, self.mock_settings, self.mock_context
        )

        self.assertEqual(res["status"], "ERROR")
        self.assertNotIn("Bigtable SQL", res["error_details"])
        self.assertEqual(
            res["error_details"], "Failed to retrieve medical records."
        )

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_sql_injection_payloads_rejected_before_execute_sql(
        self, mock_execute_sql
    ):
        injection_test_names = [
            "Glucose'] UNION SELECT * FROM patients --",
            "albumin' OR '1'='1",
            "unknown_test",
            "",
        ]
        for payload in injection_test_names:
            with self.subTest(test_name=payload):
                with self.assertRaises(ValidationError):
                    await agent_query.get_test_results_specific_test(
                        self.mock_creds,
                        self.mock_settings,
                        self.mock_context,
                        test_name=payload,
                    )

        with self.assertRaises(ValidationError):
            await agent_query.get_test_results_specific_test(
                self.mock_creds,
                self.mock_settings,
                self.mock_context,
                test_name="Glucose",
                before="2025-01-01') OR 1=1 --",
            )

        with self.assertRaises(ValidationError):
            await agent_query.get_test_results_specific_test(
                self.mock_creds,
                self.mock_settings,
                self.mock_context,
                test_name="Glucose",
                after="1970-01-01') UNION SELECT * FROM patients --",
            )

        with self.assertRaises(ValidationError):
            await agent_query.get_test_results_specific_test(
                self.mock_creds,
                self.mock_settings,
                self.mock_context,
                test_name="Glucose",
                num_tests="1) UNION SELECT * FROM patients --",
            )

        with self.assertRaises(ValidationError):
            await agent_query.get_test_results(
                self.mock_creds,
                self.mock_settings,
                self.mock_context,
                before="2025-01-01') OR 1=1 --",
            )

        with self.assertRaises(ValidationError):
            await agent_query.get_visits(
                self.mock_creds,
                self.mock_settings,
                self.mock_context,
                after="1970-01-01') OR 1=1 --",
            )

        mock_execute_sql.assert_not_called()

    @patch.object(
        agent_query.query_tool, "execute_sql", new_callable=AsyncMock
    )
    async def test_out_of_range_num_tests_rejected(self, mock_execute_sql):
        for invalid_count in [0, -1, 301, 10000]:
            with self.subTest(invalid_count=invalid_count):
                with self.assertRaises(ValidationError):
                    await agent_query.get_test_results(
                        self.mock_creds,
                        self.mock_settings,
                        self.mock_context,
                        num_tests=invalid_count,
                    )
                with self.assertRaises(ValidationError):
                    await agent_query.get_test_results_specific_test(
                        self.mock_creds,
                        self.mock_settings,
                        self.mock_context,
                        test_name="Glucose",
                        num_tests=invalid_count,
                    )

        mock_execute_sql.assert_not_called()


if __name__ == "__main__":
    unittest.main()
