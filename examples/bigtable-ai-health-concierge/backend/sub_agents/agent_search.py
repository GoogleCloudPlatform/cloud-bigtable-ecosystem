from google.adk.agents import Agent
from google.adk.tools import google_search

import os
import sys
sys.path.append(os.path.join(os.path.dirname(__file__), '..'))
from vertex_gemini import VertexGemini

Agent_Search = Agent(
    model=VertexGemini(model='gemini-2.5-flash'),
    name='SearchAgent',
    instruction="""
    You're a specialist in using Google Search for general health knowledge like when to see a doctor, symptoms or when a certain routine test should be done and finding nearby healthcare facilities and addresses of hospitals. 
    """,
    tools=[google_search]
)
