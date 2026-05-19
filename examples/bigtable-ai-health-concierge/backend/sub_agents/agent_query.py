import os
import google.auth
from google.auth.credentials import Credentials
from google.adk.agents import Agent
from typing import Literal
from google.adk.tools.bigtable.bigtable_toolset import BigtableToolset
from google.adk.tools.bigtable.settings import BigtableToolSettings
from google.adk.tools.bigtable import BigtableCredentialsConfig
from google.adk.tools.tool_context import ToolContext
from google.adk.tools.google_tool import GoogleTool
from google.adk.tools.bigtable import query_tool
from dotenv import load_dotenv
import time
from datetime import datetime
from pydantic import validate_call

import sys
sys.path.append(os.path.join(os.path.dirname(__file__), '..'))
from vertex_gemini import VertexGemini

load_dotenv(os.path.join(os.path.dirname(__file__), '../../.env'))

PROJECT_ID = os.getenv("GOOGLE_CLOUD_PROJECT")
LOCATION = os.getenv("GOOGLE_CLOUD_LOCATION")
BIGTABLE_INSTANCE_ID = os.getenv("BIGTABLE_INSTANCE_ID")


tool_settings = BigtableToolSettings()
application_default_credentials, _ = google.auth.default()
credentials_config = BigtableCredentialsConfig(
    credentials=application_default_credentials
)
bigtable_toolset = BigtableToolset(
    credentials_config=credentials_config, 
    bigtable_tool_settings=tool_settings
)

@validate_call(config={"arbitrary_types_allowed": True})
async def get_test_results(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    before:datetime = datetime.max, 
    after:datetime = datetime.fromisoformat("1970-01-01T00:00:00Z"),
    num_tests:int = 60
):
    """Returns a list of all medical tests such as blood pressure, glucose, globulin, bilirubin, albumin, co2, creatine kinase blood tests, their results and dates in the provided date range sorted in reverse chronological order (newest first). 
        Args:
        before (datetime): The ISO 8601 datetime before which the tests should be returned.
        after (datetime): The ISO 8601 datetime after which the tests should be returned.
        num_tests (int): The number of test results to return in reverse chronological order. Set to 1 for the most recent test result.
    """
    query = f"SELECT tests as results, DATE(_timestamp) as date FROM UNPACK((SELECT tests FROM `patients`(WITH_HISTORY=>TRUE, before=>TIMESTAMP('{before}'), after=>TIMESTAMP('{after}'), latest_n=>{num_tests}) WHERE _key='john.doe@gmail.com')) ORDER BY _timestamp DESC LIMIT 300"  
    res = await query_tool.execute_sql(
        project_id=PROJECT_ID,
        instance_id=BIGTABLE_INSTANCE_ID,
        query=query,
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
    )
    return res

async def get_prescriptions(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
):
    """Returns a list of prescriptions, refill dates in YYYY-MM-DD format and doctor notes."""
    query = f"SELECT prescriptions FROM patients(WITH_HISTORY=>FALSE) WHERE _key='john.doe@gmail.com' LIMIT 300"  
    res = await query_tool.execute_sql(
        project_id=PROJECT_ID,
        instance_id=BIGTABLE_INSTANCE_ID,
        query=query,
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
    )
    return res

@validate_call(config={"arbitrary_types_allowed": True})
async def get_visits(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    before:datetime = datetime.max, 
    after:datetime = datetime.fromisoformat("1970-01-01 00:00:00Z"),
):
    """Returns a list of doctor or hospital visits, procedures, screenings, shots, vaccinations, including past and upcoming events, with doctor name, facility, reason for visit, date of visit and outcome/recommendation in the provided date range sorted in reverse chronological order (newest first). 
    Speciality covers medical specialities like cardiology, dermatology, neurology, orthopedics, etc. 
        Args:
        before (datetime): The ISO 8601 datetime before which the visits should be returned.
        after (datetime): The ISO 8601 datetime after which the visits should be returned.
    """
    query = f"SELECT visits, DATE(_timestamp) as date FROM UNPACK((SELECT visits FROM patients(WITH_HISTORY=>TRUE, before=>TIMESTAMP('{before}'), after=>TIMESTAMP('{after}')) WHERE _key='john.doe@gmail.com')) ORDER BY _timestamp DESC LIMIT 300" 
    res = await query_tool.execute_sql(
        project_id=PROJECT_ID,
        instance_id=BIGTABLE_INSTANCE_ID,
        query=query,
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
    )
    return res


async def get_test_results_specific_test(
    credentials: Credentials,
    settings: BigtableToolSettings,
    tool_context: ToolContext,
    test_name: Literal[ "all", "CO2", "Glucose", "albumin", "bilirubin", "creatine_kinase", "globulin" ],
    before:datetime = datetime.max, 
    after:datetime = datetime.fromisoformat("1970-01-01T00:00:00Z"),
    num_tests:int = 60,
):
    """Returns a list of a specific medical test including blood tests, their results and dates in the provided date range sorted in reverse chronological order (newest first). 
        Args:
        test_name (str): The name of the test to return. Valid values are "all", "CO2", "Glucose", "albumin", "bilirubin", "creatine_kinase", "globulin".
        before (datetime): The ISO 8601 datetime before which the tests should be returned.
        after (datetime): The ISO 8601 datetime after which the tests should be returned.
        num_tests (int): The number of test results to return in reverse chronological order. Set to 1 for the most recent test result.
    """
    query = f"SELECT {'tests' if test_name == 'all' else 'tests['+test_name+']'} as results, DATE(_timestamp) as date FROM UNPACK((SELECT tests FROM `patients`(WITH_HISTORY=>TRUE, before=>TIMESTAMP('{before}'), after=>TIMESTAMP('{after}'), latest_n=>{num_tests}) WHERE _key='john.doe@gmail.com')) ORDER BY _timestamp DESC LIMIT 300"   
    res = await query_tool.execute_sql(
        project_id=PROJECT_ID,
        instance_id=BIGTABLE_INSTANCE_ID,
        query=query,
        credentials=credentials,
        settings=settings,
        tool_context=tool_context,
    )
    return res

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
    tools=[bigtable_toolset, 
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
            )]
)
