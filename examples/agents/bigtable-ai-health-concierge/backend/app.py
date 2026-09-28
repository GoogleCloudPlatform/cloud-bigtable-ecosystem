import argparse
import asyncio
import os
import uuid

# Allow insecure transport for local development (Google OAuth requires this for HTTP)
os.environ['OAUTHLIB_INSECURE_TRANSPORT'] = '1'

# Optimize gRPC concurrency and polling for local single-threaded Execution
os.environ['GRPC_ENABLE_FORK_SUPPORT'] = '1'
os.environ['GRPC_VERBOSITY'] = 'ERROR'

# Explicitly disable mTLS certificate discovery and exponential tenacity retry loops
os.environ['GOOGLE_API_USE_CLIENT_CERTIFICATE'] = 'false'
os.environ['GOOGLE_API_USE_MTLS_ENDPOINT'] = 'never'

from flask import Flask, jsonify, request, session, redirect
from flask_cors import CORS
from auth import authenticate_user, callback_handler
from agentscaffold import chat_with_agent
from dotenv import load_dotenv

load_dotenv(os.path.join(os.path.dirname(__file__), '../.env'))

app = Flask(__name__)
app.secret_key = os.getenv("FLASK_SECRET_KEY", "super-secret-key-for-btagent")
CORS(app, supports_credentials=True, origins=["http://localhost:3000", "http://127.0.0.1:3000"])

# Optional identity parameter (can be set via --user CLI flag or DEMO_PATIENT_EMAIL in .env)
DEMO_PATIENT_EMAIL = os.getenv("DEMO_PATIENT_EMAIL", "").strip() or None


def _format_display_name(email: str) -> str:
    return email.split('@')[0].replace('.', ' ').title()


def _resolve_identity():
    """Returns (name, email) from ?user= query param, active session, or CLI parameter."""
    user_param = request.args.get('user', '').strip()
    if user_param:
        session.pop('force_login', None)
        session['email'] = user_param
        session['name'] = _format_display_name(user_param)
        return session['name'], session['email']

    if 'email' in session and session['email']:
        return session.get('name', _format_display_name(session['email'])), session['email']

    if DEMO_PATIENT_EMAIL and not session.get('force_login'):
        return _format_display_name(DEMO_PATIENT_EMAIL), DEMO_PATIENT_EMAIL

    return None, None


@app.route('/auth/login')
def login():
    frontend_host = request.host.split(':')[0] if request.host else '127.0.0.1'
    user_param = request.args.get('user', '').strip()
    if user_param:
        session.pop('force_login', None)
        session['email'] = user_param
        session['name'] = _format_display_name(user_param)
        return redirect(f"http://{frontend_host}:3000/chat")

    if DEMO_PATIENT_EMAIL and not session.get('force_login'):
        return redirect(f"http://{frontend_host}:3000/chat")

    session.pop('force_login', None)
    return redirect(authenticate_user())


@app.route('/auth/callback')
def callback():
    return callback_handler()


@app.route('/api/user')
def get_user():
    name, email = _resolve_identity()
    if email:
        return jsonify({
            "name": name,
            "email": email
        })
    return jsonify({"error": "Unauthorized"}), 401


@app.route('/api/chat', methods=['POST'])
def chat():
    _, user_email = _resolve_identity()
    data = request.json or {}
    if not user_email and data.get("user"):
        user_email = data.get("user").strip()
        session['email'] = user_email
        session['name'] = _format_display_name(user_email)

    if not user_email:
        return jsonify({"error": "Unauthorized"}), 401

    message = data.get("message")
    access_token = session.get('access_token')
    refresh_token = session.get('refresh_token')

    if 'chat_session_id' not in session:
        session['chat_session_id'] = str(uuid.uuid4())
    session_id = session['chat_session_id']

    response = asyncio.run(chat_with_agent(user_email, message, access_token, refresh_token, session_id=session_id))
    return jsonify({"response": response})


@app.route('/api/logout')
def logout():
    session.clear()
    session['force_login'] = True
    return jsonify({"success": True})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Run the Personal Health Concierge backend.")
    parser.add_argument(
        "--user",
        dest="demo_user",
        default=None,
        help="Optional patient email (e.g. john.doe@gmail.com) to skip Google OAuth login.",
    )
    args = parser.parse_args()
    if args.demo_user:
        DEMO_PATIENT_EMAIL = args.demo_user.strip()

    app.run(port=5000, debug=True, threaded=False, use_reloader=False)