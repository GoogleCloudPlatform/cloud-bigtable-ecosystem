import logging
import os
import time
import uuid

from dotenv import load_dotenv
from google.adk.auth.auth_credential import AuthCredential, AuthCredentialTypes, OAuth2Auth
from google.adk.auth.credential_service.base_credential_service import BaseCredentialService
from google.adk.memory.vertex_ai_memory_bank_service import VertexAiMemoryBankService
from google.adk.runners import Runner
from google.adk.sessions import VertexAiSessionService
from google.adk.tools.openapi_tool.openapi_spec_parser.tool_auth_handler import ToolContextCredentialStore
from google.genai.types import Content, Part

from agent import create_adk_agent

load_dotenv(os.path.join(os.path.dirname(__file__), '../.env'))
os.environ.setdefault("OTEL_SDK_DISABLED", "true")

APP_NAME = "btagent"
CONCIERGE_AGENT = None

# Maps (user_email, client_session_id) -> Agent Engine session ID
_AGENT_ENGINE_SESSION_IDS: dict[tuple[str, str], str] = {}


class ScopedCredentialService(BaseCredentialService):
    def __init__(self, access_token: str, refresh_token: str):
        super().__init__()
        self.access_token = access_token
        self.refresh_token = refresh_token

    async def load_credential(self, auth_config, callback_context):
        if not self.access_token:
            return None

        # Set expiry to 1 hour in the future to bypass ADK's refresher
        future_expiry = int(time.time()) + 3600

        oauth2_auth = OAuth2Auth(
            client_id=os.getenv("GOOGLE_CLIENT_ID"),
            client_secret=os.getenv("GOOGLE_CLIENT_SECRET"),
            access_token=self.access_token,
            refresh_token=self.refresh_token,
            expires_at=future_expiry,
        )
        return AuthCredential(
            auth_type=AuthCredentialTypes.OAUTH2,
            oauth2=oauth2_auth,
        )

    async def save_credential(self, auth_config, callback_context):
        pass


async def chat_with_agent(user_email, message, access_token=None, refresh_token=None, session_id=None):
    """
    Handles a chat turn with the agent using Agent Engine Sessions and Memory Bank.
    """
    global CONCIERGE_AGENT

    if not session_id:
        session_id = str(uuid.uuid4())

    project = os.getenv("GOOGLE_CLOUD_PROJECT")
    location = os.getenv("GOOGLE_CLOUD_LOCATION", "us-central1")
    agent_engine_id = os.getenv("VERTEX_AI_AGENT_ENGINE_ID")

    # Instantiate Agent Engine Session Service and Memory Bank Service bound to the current request event loop
    session_service = VertexAiSessionService(
        project=project,
        location=location,
        agent_engine_id=agent_engine_id,
    )
    memory_service = VertexAiMemoryBankService(
        project=project,
        location=location,
        agent_engine_id=agent_engine_id,
    )

    engine_session_id = _AGENT_ENGINE_SESSION_IDS.get((user_email, session_id))
    existing_session = None
    if engine_session_id:
        try:
            existing_session = await session_service.get_session(
                app_name=APP_NAME,
                user_id=user_email,
                session_id=engine_session_id,
            )
        except Exception:
            existing_session = None

    if not existing_session:
        existing_session = await session_service.create_session(
            app_name=APP_NAME,
            user_id=user_email,
        )
        _AGENT_ENGINE_SESSION_IDS[(user_email, session_id)] = existing_session.id
        if len(_AGENT_ENGINE_SESSION_IDS) > 1000:
            _AGENT_ENGINE_SESSION_IDS.pop(next(iter(_AGENT_ENGINE_SESSION_IDS)))

    active_session_id = existing_session.id
    credential_service = ScopedCredentialService(access_token, refresh_token)

    if CONCIERGE_AGENT is None:
        CONCIERGE_AGENT = create_adk_agent()

    # Initialize the Runner with VertexAiSessionService + VertexAiMemoryBankService
    runner = Runner(
        agent=CONCIERGE_AGENT,
        app_name=APP_NAME,
        session_service=session_service,
        memory_service=memory_service,
        credential_service=credential_service,
    )

    # Prepare the message content with explicit role for Memory Bank parsing
    user_content = Content(role="user", parts=[Part(text=message)])

    state_delta = {}
    if access_token:
        from sub_agents.agent_booking import calendar_toolset

        try:
            tools = await calendar_toolset.get_tools()
        except Exception:
            tools = []

        if tools:
            cal_tool = tools[0]
            auth_scheme = cal_tool._rest_api_tool.auth_scheme
            auth_credential = cal_tool._rest_api_tool.auth_credential

            store = ToolContextCredentialStore(None)
            key = store.get_credential_key(auth_scheme, auth_credential)

            future_expiry = int(time.time()) + 3600
            oauth2_auth = OAuth2Auth(
                client_id=os.getenv("GOOGLE_CLIENT_ID"),
                client_secret=os.getenv("GOOGLE_CLIENT_SECRET"),
                access_token=access_token,
                refresh_token=refresh_token,
                expires_at=future_expiry,
            )
            cred = AuthCredential(auth_type=AuthCredentialTypes.OAUTH2, oauth2=oauth2_auth)
            state_delta[key] = cred.model_dump(mode="json")

    final_text = ""

    # Execute the agent asynchronously in the current request event loop
    events_stream = runner.run_async(
        user_id=user_email,
        session_id=active_session_id,
        new_message=user_content,
        state_delta=state_delta or None,
    )

    async for event in events_stream:
        if event.is_final_response() and event.content:
            final_text = event.content.parts[0].text

    return final_text
