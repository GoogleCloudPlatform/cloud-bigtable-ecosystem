import logging
import os
import sys
from datetime import datetime, timezone
from typing import Annotated, Any, Literal, get_args

import google.auth
from dotenv import load_dotenv
from google.adk.agents import Agent
from google.adk.tools.bigtable import BigtableCredentialsConfig, query_tool
from google.adk.tools.bigtable.settings import BigtableToolSettings
from google.adk.tools.google_tool import GoogleTool
from google.adk.tools.tool_context import ToolContext
from google.auth.credentials import Credentials
from pydantic import Field, validate_call

sys.path.append(os.path.join(os.path.dirname(__file__), '..'))
from vertex_gemini import VertexGemini

load_dotenv(os.path.join(os.path.dirname(__file__), '../../.env'))

PROJECT_ID = os.getenv("GOOGLE_CLOUD_PROJECT")
LOCATION = os.getenv("GOOGLE_CLOUD_LOCATION")
BIGTABLE_INSTANCE_ID = os.getenv("BIGTABLE_INSTANCE_ID")

# TODO(security): Derive patient key dynamically from authenticated
# session/ToolContext instead of using a static demo key (tracked in PR #261).
DEFAULT_PATIENT_KEY = b"john.doe@gmail.com"
DEFAULT_BEFORE_TIMESTAMP = datetime.max.replace(tzinfo=timezone.utc)
DEFAULT_AFTER_TIMESTAMP = datetime.fromisoformat("1970-01-01T00:00:00+00:00")
MAX_NUM_TESTS = 300

TestName = Literal[
    "all",
    "CO2",
    "Glucose",
    "albumin",
    "bilirubin",
    "creatine_kinase",
    "globulin",
]
ALLOWED_TEST_NAMES: frozenset[str] = frozenset(get_args(TestName))

tool_settings = BigtableToolSettings()
application_default_credentials, _ = google.auth.default()
credentials_config = BigtableCredentialsConfig(
    credentials=application_default_credentials
)


logger = logging.getLogger(__name__)


def _format_sql_timestamp(value: datetime) -> str:
    """Formats a validated datetime as a strict ISO 8601 UTC literal for Bigtable TVF."""
    if not isinstance(value, datetime):
        raise ValueError("Timestamp must be a datetime instance.")
    if value.tzinfo is None:
        value = value.replace(tzinfo=timezone.utc)
    return value.isoformat()


def _validate_num_tests(value: int) -> int:
    """Validates latest_n is an integer within [1, MAX_NUM_TESTS]."""
    if isinstance(value, bool) or not isinstance(value, int):
        raise ValueError("num_tests must be an integer.")
    if not 1 <= value <= MAX_NUM_TESTS:
        raise ValueError(f"num_tests must be between 1 and {MAX_NUM_TESTS}.")
    return value


async def _execute_scoped_sql(
    query: str,
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    parameters: dict[str, Any],
) -> dict[str, Any]:
    """Executes a scoped Bigtable SQL query and sanitizes error responses."""
    try:
        res = await query_tool.execute_sql(
            project_id=PROJECT_ID,
            instance_id=BIGTABLE_INSTANCE_ID,
            query=query,
            credentials=credentials,
            settings=settings,
            tool_context=tool_context,
            parameters=parameters,
        )
    except Exception:
        logger.exception("Bigtable SQL query execution failed")
        return {
            "status": "ERROR",
            "error_details": "Failed to retrieve medical records.",
        }

    if isinstance(res, dict) and res.get("status") == "ERROR":
        return {
            "status": "ERROR",
            "error_details": "Failed to retrieve medical records.",
        }
    return res


async def _query_test_results(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    test_name: TestName,
    before: datetime,
    after: datetime,
    num_tests: int,
) -> dict[str, Any]:
    """Builds and executes a parameterized Bigtable SQL query for medical tests."""
    if test_name not in ALLOWED_TEST_NAMES:
        raise ValueError(f"Unsupported test_name: {test_name!r}")

    before_iso = _format_sql_timestamp(before)
    after_iso = _format_sql_timestamp(after)
    validated_num_tests = _validate_num_tests(num_tests)

    parameters: dict[str, bytes] = {"patient_key": DEFAULT_PATIENT_KEY}
    if test_name == "all":
        projection = "tests"
    else:
        projection = "tests[@test_name]"
        parameters["test_name"] = test_name.encode("utf-8")

    # Bigtable GoogleSQL requires table-valued function arguments (before,
    # after, latest_n) to resolve to compile-time literals; values are strictly
    # validated above, while _key and test_name are bound via query parameters.
    query = (
        f"SELECT {projection} as results, DATE(_timestamp) as date "
        "FROM UNPACK((SELECT tests FROM `patients`("
        f"WITH_HISTORY=>TRUE, before=>TIMESTAMP('{before_iso}'), "
        f"after=>TIMESTAMP('{after_iso}'), latest_n=>{validated_num_tests}) "
        "WHERE _key = @patient_key)) "
        "ORDER BY _timestamp DESC LIMIT 300"
    )
    return await _execute_scoped_sql(
        query=query,
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
        parameters=parameters,
    )


@validate_call(config={"arbitrary_types_allowed": True})
async def get_test_results(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    before: datetime = DEFAULT_BEFORE_TIMESTAMP,
    after: datetime = DEFAULT_AFTER_TIMESTAMP,
    num_tests: Annotated[int, Field(ge=1, le=MAX_NUM_TESTS)] = 60,
):
    """Returns a list of all medical tests such as blood pressure, glucose, globulin, bilirubin, albumin, co2, creatine kinase blood tests, their results and dates in the provided date range sorted in reverse chronological order (newest first).

    Args:
        before (datetime): The ISO 8601 datetime before which the tests should be returned.
        after (datetime): The ISO 8601 datetime after which the tests should be returned.
        num_tests (int): The number of test results to return in reverse chronological order (1-300). Set to 1 for the most recent test result.
    """
    return await _query_test_results(
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
        test_name="all",
        before=before,
        after=after,
        num_tests=num_tests,
    )


@validate_call(config={"arbitrary_types_allowed": True})
async def get_prescriptions(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
):
    """Returns a list of prescriptions, refill dates in YYYY-MM-DD format and doctor notes."""
    query = (
        "SELECT prescriptions FROM patients(WITH_HISTORY=>FALSE) "
        "WHERE _key = @patient_key LIMIT 300"
    )
    return await _execute_scoped_sql(
        query=query,
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
        parameters={"patient_key": DEFAULT_PATIENT_KEY},
    )


@validate_call(config={"arbitrary_types_allowed": True})
async def get_visits(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    before: datetime = DEFAULT_BEFORE_TIMESTAMP,
    after: datetime = DEFAULT_AFTER_TIMESTAMP,
):
    """Returns a list of doctor or hospital visits, procedures, screenings, shots, vaccinations, including past and upcoming events, with doctor name, facility, reason for visit, date of visit and outcome/recommendation in the provided date range sorted in reverse chronological order (newest first).
    Speciality covers medical specialities like cardiology, dermatology, neurology, orthopedics, etc.

    Args:
        before (datetime): The ISO 8601 datetime before which the visits should be returned.
        after (datetime): The ISO 8601 datetime after which the visits should be returned.
    """
    before_iso = _format_sql_timestamp(before)
    after_iso = _format_sql_timestamp(after)
    query = (
        "SELECT visits, DATE(_timestamp) as date "
        "FROM UNPACK((SELECT visits FROM patients("
        f"WITH_HISTORY=>TRUE, before=>TIMESTAMP('{before_iso}'), "
        f"after=>TIMESTAMP('{after_iso}')) "
        "WHERE _key = @patient_key)) "
        "ORDER BY _timestamp DESC LIMIT 300"
    )
    return await _execute_scoped_sql(
        query=query,
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
        parameters={"patient_key": DEFAULT_PATIENT_KEY},
    )


@validate_call(config={"arbitrary_types_allowed": True})
async def get_test_results_specific_test(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    test_name: TestName,
    before: datetime = DEFAULT_BEFORE_TIMESTAMP,
    after: datetime = DEFAULT_AFTER_TIMESTAMP,
    num_tests: Annotated[int, Field(ge=1, le=MAX_NUM_TESTS)] = 60,
):
    """Returns a list of a specific medical test including blood tests, their results and dates in the provided date range sorted in reverse chronological order (newest first).

    Args:
        test_name (str): The name of the test to return. Valid values are "all", "CO2", "Glucose", "albumin", "bilirubin", "creatine_kinase", "globulin".
        before (datetime): The ISO 8601 datetime before which the tests should be returned.
        after (datetime): The ISO 8601 datetime after which the tests should be returned.
        num_tests (int): The number of test results to return in reverse chronological order (1-300). Set to 1 for the most recent test result.
    """
    return await _query_test_results(
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
        test_name=test_name,
        before=before,
        after=after,
        num_tests=num_tests,
    )


Agent_Query = Agent(
    model=VertexGemini(model='gemini-2.5-flash'),
    name='MedicalRecordsAgent',
    instruction="""
    You're a specialist who can access patient's personal health records, doctor appointments, prescriptions, in-network doctors by speciality. You *DO NOT* provide medical advice. You only provide relevant data to the user or other agents. 
    
    If asked about specialists or doctors seen for a specific condition, use `get_visits` to search past visits.
    `get_visits` tool instructions:
    Include doctor name, speciality, facility, reason for visit, date of visit and outcome/recommendations in the response even when not asked explicitly.
    This tool returns all visits in the provided date range in reverse chronological order (newest first). If user asks for a specific doctor, hospital or speciality, use the tool to return all in the desired timeframe and filter the tool response. To find upcoming visits such as the next visit, set `after` to {_time}. To find past visits such as the last or most recent visit, set `before` to {_time}. 
    When responding to questions about multiple visits or appointments, separate them with line breaks.

    `get_prescriptions` tool instructions:
    This tool returns all prescriptions not just ones that have refills left. For requests about upcoming refills, refills in the next [N] days/weeks/months/years, first calculate the difference between {_time} and date of refill in number of periods (days, weeks, months, years).
    If no refill date is available in the data for a medication asked about specifically, respond with 'No refills left. Please contact your doctor to get a new prescription.'

    `get_test_results_specific_test` tool instructions:
    This tool can return a specific test result or all test results in reverse chronological order (newest first). For example when asked for a specific test history (like Albumin), call this tool and do so without setting num_tests. For the exhaustive history of tests you can run this tool without setting num_tests to a value. When asked for latest result for all tests, set num_tests to 1. When test name isn't provided, call this tool with test_name='all'.
    """,
    tools=[
        GoogleTool(
            func=get_test_results,
            credentials_config=credentials_config,
            tool_settings=tool_settings,
        ),
        GoogleTool(
            func=get_prescriptions,
            credentials_config=credentials_config,
            tool_settings=tool_settings,
        ),
        GoogleTool(
            func=get_visits,
            credentials_config=credentials_config,
            tool_settings=tool_settings,
        ),
        GoogleTool(
            func=get_test_results_specific_test,
            credentials_config=credentials_config,
            tool_settings=tool_settings,
        ),
    ],
)
