import os
import sys
from dotenv import load_dotenv
import vertexai

def create_agent_engine():
    # Load project environment variables
    env_path = os.path.join(os.path.dirname(__file__), '../.env')
    load_dotenv(env_path)

    project_id = os.getenv("GOOGLE_CLOUD_PROJECT")
    location = os.getenv("GOOGLE_CLOUD_LOCATION", "us-central1")

    if not project_id or project_id == "your-google-cloud-project-id":
        print("ERROR: Please make sure GOOGLE_CLOUD_PROJECT is set in your .env file.")
        sys.exit(1)

    print(f"Initializing Vertex AI client for project: '{project_id}', location: '{location}'...")
    try:
        client = vertexai.Client(project=project_id, location=location)
    except Exception as e:
        print(f"Failed to initialize Vertex AI client: {e}")
        sys.exit(1)

    print("Creating empty Vertex AI Agent Engine instance (this may take a few seconds)...")
    try:
        agent_engine = client.agent_engines.create()
        # The resource name format is projects/.../locations/.../reasoningEngines/<ID>
        resource_name = agent_engine.api_resource.name
        agent_engine_id = resource_name.split("/")[-1]
        
        print("\n" + "="*60)
        print("🎉 Successfully created Vertex AI Agent Engine instance!")
        print(f"Resource Name: {resource_name}")
        print(f"Agent Engine ID: {agent_engine_id}")
        print("="*60)
        print(f"\nNext step: Add the following line to your .env file:")
        print(f"VERTEX_AI_AGENT_ENGINE_ID={agent_engine_id}\n")

    except Exception as e:
        print(f"Failed to create Agent Engine instance: {e}")
        sys.exit(1)

if __name__ == "__main__":
    create_agent_engine()
