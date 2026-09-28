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

*(Optional)* To start the backend with a pre-configured demo user identity (`john.doe@gmail.com`) so it skips the login step by default:
```bash
python app.py --user john.doe@gmail.com
```

### 3. Frontend Setup (Next.js)
```bash
cd frontend
npm install
npm run dev
```
The frontend will run on `http://127.0.0.1:3000` (or `http://localhost:3000`).

### 4. Accessing the App & Demo Login Bypass (`?user=`)

- **Standard Google OAuth Login (Forced when no identity is provided)**:
  Open `http://localhost:3000` (or `http://127.0.0.1:3000`). When no user identity is present in the session or URL, the app forces the login page where you can click **Sign in with Google** (requires `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` configured in `.env`).
- **Demo User Parameter Bypass (`?user=john.doe@gmail.com`)**:
  To skip the Google OAuth login page and sign in directly as the seeded demo patient (`john.doe@gmail.com`), pass the `user` query parameter in the URL:
  ```text
  http://localhost:3000/?user=john.doe@gmail.com
  ```
  (or `http://localhost:3000/chat?user=john.doe@gmail.com`). Clicking **Logout** in the chat header will clear the session and return you to the login screen.

## How it Works

1.  **Login & Dynamic User Identity**: Users sign in via Google OAuth (or pass `?user=john.doe@gmail.com` for demo mode). The backend resolves the active user's email and passes it into the ADK `Runner` (`user_id`), which propagates it via `CallbackContext` and `ToolContext` into all Bigtable queries.
2.  **Personalized Greeting**: Upon entering the chat, the agent greets the user by name and loads their profile from Bigtable.
3.  **Memory Bank**: Every message is passed to the ADK Agent, which uses `VertexAiMemoryBankService`. The `session_id` is scoped to the user's session, ensuring that the agent remembers specific context for that user across turns.
