import os
import requests
from flask import session, redirect, url_for, request
from google_auth_oauthlib.flow import Flow
from google.oauth2 import id_token
from google.auth.transport import requests as google_requests
from dotenv import load_dotenv

load_dotenv(os.path.join(os.path.dirname(__file__), '../.env'))

CLIENT_ID = os.getenv("GOOGLE_CLIENT_ID")
CLIENT_SECRET = os.getenv("GOOGLE_CLIENT_SECRET")
REDIRECT_URI = os.getenv("GOOGLE_CALLBACK_URL")

    
os.environ['OAUTHLIB_RELAX_TOKEN_SCOPE'] = '1'


# Scopes for Google OAuth
SCOPES = [
    "openid",
    "https://www.googleapis.com/auth/userinfo.email",
    "https://www.googleapis.com/auth/userinfo.profile",
    "https://www.googleapis.com/auth/calendar",
]

def get_google_auth_flow():
    return Flow.from_client_config(
        {
            "web": {
                "client_id": CLIENT_ID,
                "client_secret": CLIENT_SECRET,
                "auth_uri": "https://accounts.google.com/o/oauth2/auth",
                "token_uri": "https://oauth2.googleapis.com/token",
                "redirect_uris": [REDIRECT_URI],
            }
        },
        scopes=SCOPES,
    )

def authenticate_user():
    flow = get_google_auth_flow()
    flow.redirect_uri = REDIRECT_URI
    authorization_url, state = flow.authorization_url(
        access_type='offline',
        include_granted_scopes='false',
        prompt='consent'
    )
    session['state'] = state
    session['code_verifier'] = flow.code_verifier
    return authorization_url

def callback_handler():
    flow = get_google_auth_flow()
    flow.redirect_uri = REDIRECT_URI
    
    if 'code_verifier' in session:
        flow.code_verifier = session['code_verifier']
        
    authorization_response = request.url
    flow.fetch_token(authorization_response=authorization_response)
    
    credentials = flow.credentials
    request_session = google_requests.Request()
    
    id_info = id_token.verify_oauth2_token(
        credentials.id_token, request_session, CLIENT_ID
    )
    
    session['google_id'] = id_info.get("sub")
    session['name'] = id_info.get("name")
    session['email'] = id_info.get("email")
    session['id_token'] = credentials.id_token
    session['access_token'] = credentials.token
    
    # Only update refresh_token if it is provided (e.g., during consent)
    if credentials.refresh_token:
        session['refresh_token'] = credentials.refresh_token

    
    return redirect("http://127.0.0.1:3000/chat") # Redirect back to frontend
