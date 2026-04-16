import httpx

from .models import Transcript

GRAPHQL_URL = "https://api.fireflies.ai/graphql"

TRANSCRIPT_QUERY = """
query GetTranscript($id: String!) {
  transcript(id: $id) {
    id
    title
    date
    organizer_email
    participants
    meeting_attendees {
      displayName
      email
      name
    }
    speakers {
      id
      name
    }
    summary {
      action_items
      overview
      keywords
      gist
    }
    sentences {
      text
      speaker_name
      speaker_id
      start_time
      end_time
      ai_filters {
        task
      }
    }
    transcript_url
  }
}
"""


class FirefliesClient:
    def __init__(self, api_key: str):
        self._api_key = api_key
        self._http = httpx.AsyncClient(
            base_url=GRAPHQL_URL,
            headers={
                "Authorization": f"Bearer {api_key}",
                "Content-Type": "application/json",
            },
            timeout=30,
        )

    async def get_transcript(self, meeting_id: str) -> Transcript:
        resp = await self._http.post(
            GRAPHQL_URL,
            json={"query": TRANSCRIPT_QUERY, "variables": {"id": meeting_id}},
        )
        if resp.status_code != 200:
            import logging
            logging.getLogger(__name__).error("Fireflies API %d: %s", resp.status_code, resp.text[:500])
        resp.raise_for_status()
        data = resp.json()
        if errors := data.get("errors"):
            raise RuntimeError(f"Fireflies GraphQL error: {errors}")
        return Transcript.model_validate(data["data"]["transcript"])

    async def close(self):
        await self._http.aclose()
