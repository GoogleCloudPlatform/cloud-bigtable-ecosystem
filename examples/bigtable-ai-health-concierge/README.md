# ADK Agent Web Chat with Memory Bank

This project implements a personalized AI agent using Google Cloud's Agent Development Kit (ADK) and Agent Platform Memory Bank, integrated into a Next.js web application with Google OAuth 2.0 login.

## Prerequisites

1.  **Google Cloud Project**:
    *   Enable Agent Platform API.
    *   Enable Cloud Bigtable API.
    *   Enable Google Calendar API.
2.  **OAuth Credentials**:
    *   Go to [Google Cloud Console > APIs & Services > Credentials](https://console.cloud.google.com/apis/credentials).
    *   Create an "OAuth 2.0 Client ID" for a Web Application.
    *   Add `http://127.0.0.1:5000/auth/callback` to the **Authorized redirect URIs**.

## Setup Instructions

### 1. Environment Variables
Fill in the `.env` file in the root directory with your credentials:
```env
GOOGLE_CLOUD_PROJECT=your-project-id
GOOGLE_CLOUD_LOCATION=us-central1
VERTEX_AI_AGENT_ENGINE_ID=your-agent-engine-id
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_CALLBACK_URL=http://127.0.0.1:5000/auth/callback
BIGTABLE_INSTANCE_ID=your-bigtable-instance-id
```

### 2. Backend Setup (Flask)

#### 2.1 Set up dependencies
```bash
cd backend
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

#### 2.2 Create an empty Agent Platform Agent Engine instance
Run the initial provisioning script to establish a cloud backing store for Memory Bank:
```bash
python create_agent_engine.py
# Retrieve the Engine ID printed in the terminal and add it to your .env file)
```

> [!NOTE]  
> **Retrieving Agent Engine ID from the Console**  
> After running `create_agent_engine.py`, the script will automatically print your newly generated `VERTEX_AI_AGENT_ENGINE_ID` in the terminal for you to paste into your `.env` file. If you ever need to find or verify this ID later in the Google Cloud Web Console:
> 1. Go to [Google Cloud Console > Agent Platform](https://console.cloud.google.com/agent-platform).
> 2. Look in the side navigation menu under **Deployments**.
> 3. Click on your active instance. The ID is the last numeric segment of the full instance resource name (e.g. `1234567890123456`).


#### 2.3 Initialize Bigtable and seed demo data
```bash
python setup_demo_bigtable.py
```

#### 2.4 Start the backend server
```bash
python app.py
```
The backend will run on `http://127.0.0.1:5000`.




### 3. Frontend Setup (Next.js)
```bash
cd frontend
npm install
npm run dev
```
The frontend will run on `http://127.0.0.1:3000`.

## How it Works

1.  **Login**: Users sign in via Google. The backend handles the OAuth flow and retrieves the user's name and ID.
2.  **Personalized Greeting**: Upon entering the chat, the agent greets the user by name (extracted from Google profile).
3.  **Memory Bank**: Every message is passed to the ADK Agent, which uses `VertexAiMemoryBankService`. The `session_id` is set to the user's Google ID, ensuring that the agent remembers specific context for that user across different turns and sessions.
