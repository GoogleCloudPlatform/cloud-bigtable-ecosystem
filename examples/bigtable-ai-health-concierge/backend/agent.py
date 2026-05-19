import os
from datetime import datetime
import google.auth
from google.auth.credentials import Credentials
from google.adk.agents.callback_context import CallbackContext
from google.adk.agents import Agent
from google.adk.models import Gemini
from google.adk.tools import agent_tool
from google.adk.tools.bigtable.settings import BigtableToolSettings
from google.adk.tools.bigtable import BigtableCredentialsConfig
from google.adk.tools.tool_context import ToolContext
from google.adk.tools.google_tool import GoogleTool
from google.adk.tools.bigtable import query_tool
from dotenv import load_dotenv
from model_armor_guard import create_model_armor_guard
from vertex_gemini import VertexGemini
import json

from sub_agents.agent_search import Agent_Search
from sub_agents.agent_query import Agent_Query
from sub_agents.agent_booking import Agent_Booking
from google.adk.tools.preload_memory_tool import PreloadMemoryTool
from google.adk.tools.load_memory_tool import LoadMemoryTool


load_dotenv(os.path.join(os.path.dirname(__file__), '../.env'))

PROJECT_ID = os.getenv("GOOGLE_CLOUD_PROJECT")
LOCATION = os.getenv("GOOGLE_CLOUD_LOCATION")
BIGTABLE_INSTANCE_ID = os.getenv("BIGTABLE_INSTANCE_ID")

tool_settings = BigtableToolSettings()
application_default_credentials, _ = google.auth.default()
credentials_config = BigtableCredentialsConfig(
    credentials=application_default_credentials
)



def update_time(callback_context: CallbackContext):
  # get current date time
  now = datetime.now()
  formatted_time = now.strftime("%Y-%m-%d %H:%M:%S")
  callback_context.state["_time"] = formatted_time


async def get_profile_info(callback_context: CallbackContext):
    """Returns the patient's demographic information such as age, gender, home zip code, and work zip code to help personalize responses. Use zip codes when searching for nearby medical facilities and pharmacies."""
    if callback_context.state.get("_patient_demographics"):
        return None
    query = f"SELECT profile FROM patients WHERE _key='john.doe@gmail.com'"  
    res = await query_tool.execute_sql(
        project_id=PROJECT_ID,
        instance_id=BIGTABLE_INSTANCE_ID,
        query=query,
        credentials=application_default_credentials,
        settings=tool_settings,
        tool_context=ToolContext(invocation_context=callback_context)
    )
    callback_context.state["_patient_demographics"] = json.dumps(res['rows'][0])


async def generate_memories_callback(callback_context: CallbackContext):
    """Extracts user preferences and context from recent events and updates Memory Bank in the background."""
    try:
        import copy
        from google.adk.sessions.session import Session
        
        # 1. Create a clean, E2E-aligned session object
        clean_session = Session(
            app_name=callback_context.session.app_name,
            user_id=callback_context.session.user_id,
            id=callback_context.session.id
        )
        
        # 2. Extract clean text turns (filtering out empty intermediate events)
        for event in callback_context.session.events:
            if event.content and event.content.parts:
                has_text = False
                for part in event.content.parts:
                    if hasattr(part, 'text') and part.text and part.text.strip():
                        has_text = True
                        break
                if has_text:
                    clean_event = copy.deepcopy(event)
                    
                    # Normalize authors strictly to "user" and "model"
                    if clean_event.content.role == "user":
                        clean_event.author = "user"
                        # Strip any leading/trailing literal quotes from prompt text
                        for part in clean_event.content.parts:
                            if part.text:
                                text = part.text.strip()
                                if text.startswith('"') and text.endswith('"'):
                                    part.text = text[1:-1]
                    elif clean_event.content.role == "model":
                        clean_event.author = "model"
                        
                    clean_session.events.append(clean_event)

        # 3. Synchronously dispatch and await cloud delivery using wait_for_completion
        if clean_session.events:
            await callback_context.add_events_to_memory(
                events=clean_session.events,
                custom_metadata={"wait_for_completion": True}
            )
    except Exception:
        pass
    return None






def create_adk_agent() -> Agent:
    model_armor_guard = create_model_armor_guard()
    return Agent(
        name="PersonalHealthConcierge",
        instruction=(
            f"You are a helpful assistant with access to several tools including Google Calendar to retrieve information about personal health records, doctor appointments, prescriptions, in-network doctors by speciality and to search for general health knowledge and user's calendar availability for appointments."
            "You are also equipped with long-term memory persistence. When the user shares personal preferences, schedule constraints, or details they ask to be remembered (such as favorite colors or pharmacy locations), warmly acknowledge the request and confirm you will remember them."
            "The current date and time is {_time}. The patient's profile information is {_patient_demographics}."
            "Use the context such as age, gender and medical history to personalize conversations including responses you generate by calling the SearchAgent."
            "This is a demo application and does not store any real personal health records.These are dummy records created for demo purposes."
            "Always pass information about time frames to the tools to get the most relevant information. For example, if the user asks about doctor visits in the past 12 months, pass 'past 12 months' as the time frame to the tool. Pass temporal context like next, upcoming, last, latest, most recent etc. to the tools."
            "Always calculate the difference between {_time} and dates from MedicalRecordsAgent tool to determine if a visit, appointment, shot, procedure or test is in the past or future to answer recency related questions like last test, most recent shot, upcoming appointment, etc."
            "You can chain the tools for complex reasoning. For example, to answer a question like 'Are there any screenings I should be thinking about this year?', you can use 1) Agent_Query tool to get the patient's history and 2) Agent_Search tool to search for screenings based on the patient's age then use history from Step 1 to filter screenings that are not due based on recency."
            "For example, for a question like 'Do I need to fast for my next appointment?', you can use 1) Agent_Query tool to get the patient's appointments and 2) Agent_Search tool to search for whether any of the tests in that appointment require fasting."
            "Format dates in long date format and show them in bold."
            "If the user asks about tests without asking for a specific test, always respond with markdown tables and refuse to respond with a chart even if the user requests it as it will be too cluttered and hard to interpret with a chart."
            "In tables, split output results that look like JSON ({test:result} pairs) into separate columns for test and result."
            "When responses are not in tabular format, medication names, doctor names, facility names and dates should be in bold."
            "When you need to show the history or time series for a test, you **MUST** output a ```recharts code block containing a valid JSON array of objects. Each object should have 'date' and 'value' keys with dates sorted in ascending order. For example:\n"
            "```recharts\n"
            "[\n"
            "  {\"date\": \"2024-01-01\", \"value\": 90},\n"
            "  {\"date\": \"2024-02-01\", \"value\": 85}\n"
            "]\n"
            "```\n"
            "Unless explicitly asked to do so, **DO NOT** include tables or text with data in your response if the response contains a recharts block."
            "**DO NOT** provide typical ranges for the tests unless explicitly asked."
            "Replace underscores with spaces in names for readability and capitalize the first letter for each word in names, first letter of values in each column when presenting results in tabular format."
            "For responses that are neither tables nor recharts, respond in a conversational tone and be friendly. Use sentences instead of verbatim field names returned by the tools e.g. Prefer 'Your Lisinopril refill date is June 11, 2026' instead of 'Lisinopril: Refill Date: June 11, 2026'"
        ),
        model=VertexGemini(model="gemini-2.5-flash"),
        tools=[agent_tool.AgentTool(agent=Agent_Search), 
        agent_tool.AgentTool(agent=Agent_Query), 
        agent_tool.AgentTool(agent=Agent_Booking),
        PreloadMemoryTool(),
        LoadMemoryTool()],
        before_agent_callback=[update_time, get_profile_info],
        after_agent_callback=generate_memories_callback,
        before_model_callback=model_armor_guard.before_model_callback,
        after_model_callback=model_armor_guard.after_model_callback,
    )

PersonalHealthConcierge = create_adk_agent()
root_agent = PersonalHealthConcierge
