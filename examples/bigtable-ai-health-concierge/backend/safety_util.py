"""Utility functions for Model Armor."""
import logging
from typing import Any

from google.cloud.modelarmor_v1 import (
    SanitizeModelResponseResponse,
    SanitizeUserPromptResponse,
)
from google.cloud.modelarmor_v1.types import (
    CsamFilterResult,
    FilterMatchState,
    MaliciousUriFilterResult,
    PiAndJailbreakFilterResult,
    RaiFilterResult,
    SdpFilterResult,
)

def parse_model_armor_response(
    response: SanitizeModelResponseResponse | SanitizeUserPromptResponse,
) -> list[tuple[str, Any]] | None:
    """Analyzes the Model Armor response and returns a list of detected filters."""
    sanitization_result = response.sanitization_result

    if (
        not sanitization_result
        or sanitization_result.filter_match_state == FilterMatchState.NO_MATCH_FOUND
    ):
        return None

    detected_filters = []
    filter_matches = sanitization_result.filter_results

    if "csam" in filter_matches:
        detected_filters.extend(
            parse_csam_filter(filter_matches["csam"].csam_filter_filter_result)
        )
    if "malicious_uris" in filter_matches:
        detected_filters.extend(
            parse_malicious_uris_filter(
                filter_matches["malicious_uris"].malicious_uri_filter_result
            )
        )
    if "rai" in filter_matches:
        detected_filters.extend(
            parse_rai_filter(filter_matches["rai"].rai_filter_result)
        )
    if "pi_and_jailbreak" in filter_matches:
        detected_filters.extend(
            parse_pi_and_jailbreak_filter(
                filter_matches[
                    "pi_and_jailbreak"
                ].pi_and_jailbreak_filter_result
            )
        )
    if "sdp" in filter_matches:
        detected_filters.extend(
            parse_sdp_filter(filter_matches["sdp"].sdp_filter_result)
        )

    logging.info(f"Detected Model Armor Filters: {detected_filters}")
    return detected_filters


def parse_csam_filter(csam_result: CsamFilterResult) -> list[str]:
    if csam_result.match_state == FilterMatchState.MATCH_FOUND:
        return ["CSAM"]
    return []


def parse_malicious_uris_filter(
    uri_result: MaliciousUriFilterResult,
) -> list[str]:
    if uri_result.match_state == FilterMatchState.MATCH_FOUND:
        return ["Malicious URIs"]
    return []


def parse_rai_filter(rai_result: RaiFilterResult) -> list[str]:
    if rai_result.match_state == FilterMatchState.MATCH_FOUND:
        return [
            filter_name
            for filter_name, matched in rai_result.rai_filter_type_results.items()
            if matched.match_state == FilterMatchState.MATCH_FOUND
        ]

    return []


def parse_pi_and_jailbreak_filter(
    pi_result: PiAndJailbreakFilterResult,
) -> list[str]:
    if pi_result.match_state == FilterMatchState.MATCH_FOUND:
        return ["Prompt Injection and Jailbreaking"]
    return []


def parse_sdp_filter(sdp_result: SdpFilterResult) -> list[str]:
    detected_filters = []
    inspect_result = sdp_result.inspect_result
    if (
        inspect_result
        and inspect_result.match_state == FilterMatchState.MATCH_FOUND
    ):
        for finding in inspect_result.findings:
            info_type = finding.info_type.replace("_", " ").capitalize()
            detected_filters.append(info_type)
    deidentify_result = sdp_result.deidentify_result
    if (
        deidentify_result
        and deidentify_result.match_state == FilterMatchState.MATCH_FOUND
    ):
        for info_type in deidentify_result.info_types:
            formatted_info_type = info_type.replace("_", " ").capitalize()
            detected_filters.append(formatted_info_type)

    return detected_filters
