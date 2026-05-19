import os

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

@app.route('/auth/login')
def login():
    return redirect(authenticate_user())

@app.route('/auth/callback')
def callback():
    return callback_handler()

@app.route('/api/user')
def get_user():
    if 'name' in session:
        return jsonify({
            "name": session['name'],
            "email": session['email']
        })
    return jsonify({"error": "Unauthorized"}), 401

@app.route('/api/chat', methods=['POST'])
def chat():
    if 'name' not in session:
        return jsonify({"error": "Unauthorized"}), 401
    
    data = request.json
    message = data.get("message")
    user_name = session['name']
    #user_id = session['google_id']
    user_email = os.getenv("DEMO_PATIENT_EMAIL", "john.doe@gmail.com")
    access_token = session.get('access_token')
    refresh_token = session.get('refresh_token')
    
    import asyncio
    response = asyncio.run(chat_with_agent(user_email, message, access_token, refresh_token))
    return jsonify({"response": response})

@app.route('/api/logout')
def logout():
    session.clear()
    return jsonify({"success": True})


if __name__ == '__main__':
    app.run(port=5000, debug=True, threaded=False, use_reloader=False)
