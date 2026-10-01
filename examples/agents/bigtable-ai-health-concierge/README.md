# ADK Agent Web Chat with Cloud Bigtable & Memory Bank

This project implements a personalized AI Health Concierge agent using Google Cloud's Agent Development Kit (ADK), **Cloud Bigtable**, and **Agent Platform (Agent Engine Sessions & Memory Bank)**, integrated into a Next.js web application with Google OAuth 2.0 login and a built-in demo user bypass.

## Prerequisites

1. **Google Cloud Project**:
   * Enable the **Agent Platform API**.
   * Enable the **Cloud Bigtable API** (`bigtable.googleapis.com`) and **Cloud Bigtable Admin API** (`bigtableadmin.googleapis.com`).
   * Enable the **Google Calendar API** (`calendar-json.googleapis.com`) *(optional, required only if using Google Calendar integration)*.
   * Create a Cloud Bigtable instance in your project.

2. **Local Google Cloud Authentication (ADC)**:
   Authenticate your local environment so the backend can access Cloud Bigtable and Agent Platform:
   ```bash
   gcloud auth application-default login
   ```

3. **OAuth 2.0 Credentials *(Optional if using Demo User Bypass)***:
   * Go to [Google Cloud Console > APIs & Services > Credentials](https://console.cloud.google.com/apis/credentials).
   * Create an **OAuth 2.0 Client ID** for a **Web application**.
   * Add `http://127.0.0.1:5000/auth/callback` (or `http://localhost:3000/auth/callback`) to **Authorized redirect URIs**.

---

## Setup Instructions

### 1. Environment Variables
Copy the example environment file in the project root (`examples/agents/bigtable-ai-health-concierge/`) and fill in your configuration:
```bash
cp .env.example .env
```

Edit `.env`:
```env
GOOGLE_CLOUD_PROJECT=your-project-id
GOOGLE_CLOUD_LOCATION=us-central1
VERTEX_AI_AGENT_ENGINE_ID=your-agent-engine-id
BIGTABLE_INSTANCE_ID=your-bigtable-instance-id

# Required for Google OAuth Login (optional when using ?user=john.doe@gmail.com bypass)
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_CALLBACK_URL=http://127.0.0.1:5000/auth/callback

# Optional: Set a default demo user if you always want to bypass OAuth on the backend
# DEFAULT_USER_EMAIL=john.doe@gmail.com
```

### 2. Backend Setup (Flask)

#### 2.1 Install Python dependencies
```bash
cd backend
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

#### 2.2 Create an Agent Platform Agent Engine instance (for Sessions & Memory Bank)
Run the provisioning script to create an Agent Engine instance that acts as the cloud backing store for Sessions and Memory Bank:
```bash
python create_agent_engine.py
```
Copy the `VERTEX_AI_AGENT_ENGINE_ID` printed in the terminal and set it in your `.env` file.

> [!NOTE]
> **Retrieving Agent Engine ID from the Console**
> 1. Go to [Google Cloud Console > Agent Platform](https://console.cloud.google.com/agent-platform).
> 2. Look in the side navigation menu under **Deployments** (or **Agent Engine**).
> 3. Click on your active `health_concierge_memory_engine` instance. The ID is the last numeric segment of the full instance resource name (e.g. `1234567890123456`).

#### 2.3 Initialize Cloud Bigtable and seed demo data
Create the required Bigtable tables (`user_profiles`, `wearable_metrics`, `health_knowledge`) and seed sample health records for the demo user (`john.doe@gmail.com`):
```bash
python setup_demo_bigtable.py
```

#### 2.4 Start the backend server
```bash
python app.py
```
*(Or run `python app.py --user john.doe@gmail.com` to force the backend to default to the demo user).*

The backend server listens on `http://127.0.0.1:5000`.

### 3. Frontend Setup (Next.js)
In a separate terminal:
```bash
cd frontend
npm install
npm run dev
```
The frontend runs on `http://localhost:3000` and automatically proxies `/api/*` and `/auth/*` requests to the Flask backend on `http://127.0.0.1:5000`.

---

## Logging In: Demo User Bypass vs. Google OAuth

### Option 1: Demo User Bypass (`john.doe@gmail.com`)
To test the agent immediately with the pre-seeded Cloud Bigtable dataset without configuring Google OAuth:
* Open **`http://localhost:3000/?user=john.doe@gmail.com`** in your browser.
* Passing `?user=<email>` skips the login page, sets the active session user to `john.doe@gmail.com`, and opens the chat interface directly.
* Alternatively, you can launch the backend with `python app.py --user john.doe@gmail.com` (or set `DEFAULT_USER_EMAIL=john.doe@gmail.com` in `.env`) and visit `http://localhost:3000/`.

### Option 2: Google OAuth 2.0 Login
* Open **`http://localhost:3000/`** in your browser.
* If no active session or `?user=` query parameter is present, the app displays the **Sign in with Google** screen.
* Once authenticated, the user's Google email and profile name are stored in the session and passed into ADK session state (`user_email`, `user_name`).

---

## How It Works

1. **Dynamic User Identity in Cloud Bigtable Queries**:
   * Upon entering the chat, the backend passes the authenticated user's email into the ADK session state (`user_email`).
   * Both the root agent's profile lookup (`get_profile_info`) and the Bigtable sub-agent (`get_wearable_metrics`, `get_health_knowledge`) dynamically read `user_email` from ADK's `CallbackContext` / `ToolContext` to query Cloud Bigtable rows scoped to that specific user (`WHERE _key = '<user_email>'`).

2. **Persistent Sessions & Long-Term Memory (Agent Engine & Memory Bank)**:
   * **Session Persistence (`VertexAiSessionService`)**: Persists conversation sessions, turn-by-turn events, and session state in your Agent Engine instance (`VERTEX_AI_AGENT_ENGINE_ID`) rather than ephemeral local memory.
   * **Long-Term Memory (`VertexAiMemoryBankService`)**:
     * **Memory Extraction (`after_agent_callback`)**: After each agent turn, `generate_memories_callback` triggers `memory_service.add_session_to_memory(session)` asynchronously (`wait_for_completion=False`), instructing Memory Bank to extract and consolidate meaningful long-term user facts and preferences under `{app_name: "btagent", user_id: "<user_email>"}`.
     * **Memory Retrieval (`PreloadMemoryTool` & `LoadMemoryTool`)**: On subsequent turns and new sessions, the agent automatically preloads and searches the user's Memory Bank to personalize responses.

---

## Testing & Verifying Memory Bank

1. **Log in as the demo user**:
   Open `http://localhost:3000/?user=john.doe@gmail.com`.
2. **Verify Bigtable data retrieval**:
   Ask:
   > *"How have my sleep and HRV been over the past few days, and what are my current health goals?"*
3. **Store new long-term memories**:
   Tell the agent facts or preferences you want it to remember across sessions:
   > *"Please remember that my preferred pharmacy is Walgreens on Market Street, I am allergic to penicillin, and I only like morning appointments before 10 AM."*
4. **Verify in the Google Cloud Console**:
   * Go to [Google Cloud Console > Agent Platform](https://console.cloud.google.com/agent-platform) and select your Agent Engine deployment (`VERTEX_AI_AGENT_ENGINE_ID`).
   * Open the **Sessions** tab to inspect persisted conversation turns for `john.doe@gmail.com`.
   * Open the **Memories** tab (allow ~15–30 seconds after the turn completes for background consolidation) to see the extracted memory entries scoped to `john.doe@gmail.com`.
5. **Test cross-session recall**:
   Restart the backend or open a fresh browser window at `http://localhost:3000/?user=john.doe@gmail.com` and ask:
   > *"What allergies do I have, which pharmacy do I prefer, and when should you schedule my appointments?"*

