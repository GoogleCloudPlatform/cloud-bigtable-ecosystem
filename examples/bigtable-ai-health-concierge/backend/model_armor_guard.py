import os
from google.cloud import modelarmor_v1
from google.api_core.client_options import ClientOptions
from safety_util import parse_model_armor_response
from google.adk.models import LlmRequest, LlmResponse
from google.genai.types import Content, Part

class ModelArmorGuard:
    def __init__(self, template_name: str, location: str, block_on_match: bool = True):
        self.template_name = template_name
        self.block_on_match = block_on_match
        self.is_enabled = bool(os.getenv("MODEL_ARMOR_TEMPLATE_NAME"))
        self.client = modelarmor_v1.ModelArmorClient(
            transport="rest",
            client_options=ClientOptions(
                api_endpoint=f"modelarmor.{location}.rep.googleapis.com"
            ),
        )

    def _extract_user_text(self, llm_request: LlmRequest) -> str:
        if not llm_request or not llm_request.contents:
            return ""
        
        texts = []
        for content in llm_request.contents:
            if content.parts:
                for part in content.parts:
                    if hasattr(part, 'text') and part.text:
                        texts.append(part.text)
        return " ".join(texts)
    
    def _extract_model_text(self, llm_response: LlmResponse) -> str:
        if not llm_response or not llm_response.content or not llm_response.content.parts:
            return ""
        
        texts = []
        for part in llm_response.content.parts:
            if hasattr(part, 'text') and part.text:
                texts.append(part.text)
        return " ".join(texts)

    def _get_matched_filters(self, result) -> list[str]:
        return parse_model_armor_response(result) or []

    def before_model_callback(self, llm_request: LlmRequest, **kwargs) -> LlmResponse | None:
        if not self.is_enabled:
            return None

        user_text = self._extract_user_text(llm_request)
        if not user_text:
            return None

        sanitize_request = modelarmor_v1.SanitizeUserPromptRequest(
            name=self.template_name,
            user_prompt_data=modelarmor_v1.DataItem(text=user_text),
        )
        result = self.client.sanitize_user_prompt(request=sanitize_request)
        
        matched_filters = self._get_matched_filters(result)
        if matched_filters and self.block_on_match:
            print(f"[ModelArmorGuard] 🛡️ BLOCKED - Threats detected: {matched_filters}")
            
            if 'Prompt Injection and Jailbreaking' in matched_filters:
                message = (
                    "I apologize, but I cannot process this request. "
                    "Your message appears to contain instructions that could "
                    "compromise my safety guidelines. Please rephrase your question."
                )
            elif any('sdp' in f.lower() or 'ssn' in f.lower() or 'credit' in f.lower() for f in matched_filters) or any(f.lower().startswith('us_') for f in matched_filters):
                message = (
                    "I noticed your message contains sensitive personal information "
                    "(like SSN or credit card numbers). For your security, I cannot "
                    "process requests containing such data. Please remove the sensitive "
                    "information and try again."
                )
            elif any(f.lower() in ["harm category hate speech", "harm category dangerous content", "harm category harassment", "harm category sexually explicit", "csam"] for f in matched_filters):
                message = (
                    "I apologize, but I cannot respond to this type of request. "
                    "Please rephrase your question in a respectful manner, and "
                    "I'll be happy to help."
                )
            else:
                message = (
                    "I apologize, but I cannot process this request due to "
                    "security concerns. Please rephrase your question."
                )
                
            return LlmResponse(
                content=Content(
                    role="model",
                    parts=[Part.from_text(text=message)]
                )
            )
            
        print(f"[ModelArmorGuard] ✅ User prompt passed security screening")
        return None

    def after_model_callback(self, llm_response: LlmResponse, **kwargs) -> LlmResponse | None:
        if not self.is_enabled:
            return None

        model_text = self._extract_model_text(llm_response)
        if not model_text:
            return None

        sanitize_request = modelarmor_v1.SanitizeModelResponseRequest(
            name=self.template_name,
            model_response_data=modelarmor_v1.DataItem(text=model_text),
        )
        result = self.client.sanitize_model_response(request=sanitize_request)
        
        matched_filters = self._get_matched_filters(result)
        if matched_filters and self.block_on_match:
            print(f"[ModelArmorGuard] 🛡️ Response sanitized - Issues detected: {matched_filters}")
            message = (
                "I apologize, but my response was filtered for security reasons. "
                "Could you please rephrase your question? I'm here to help with "
                "your customer service needs."
            )
            return LlmResponse(
                content=Content(
                    role="model",
                    parts=[Part.from_text(text=message)]
                )
            )
            
        print(f"[ModelArmorGuard] ✅ Model response passed security screening")
        return None

def create_model_armor_guard() -> ModelArmorGuard:
    project_id = os.getenv("GOOGLE_CLOUD_PROJECT", "your-project-id")
    location = os.getenv("GOOGLE_CLOUD_LOCATION", "us-central1")
    template_name = os.getenv("MODEL_ARMOR_TEMPLATE_NAME", f"projects/{project_id}/locations/{location}/templates/Bigtable_demo")
    
    return ModelArmorGuard(
        template_name=template_name,
        location=location
    )
