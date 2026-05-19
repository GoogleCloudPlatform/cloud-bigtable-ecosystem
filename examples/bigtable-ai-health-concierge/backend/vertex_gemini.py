import os
from google.adk.models import Gemini
from google.genai import Client

# Explicitly disable mTLS certificate discovery and exponential tenacity retry loops
os.environ['GOOGLE_API_USE_CLIENT_CERTIFICATE'] = 'false'
os.environ['GOOGLE_API_USE_MTLS_ENDPOINT'] = 'never'

class VertexGemini(Gemini):
    @property
    def api_client(self) -> Client:
        project = os.getenv("GOOGLE_CLOUD_PROJECT")
        location = os.getenv("GOOGLE_CLOUD_LOCATION", "us-central1")
        return Client(vertexai=True, project=project, location=location)
