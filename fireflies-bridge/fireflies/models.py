from pydantic import BaseModel


class Speaker(BaseModel):
    id: str | int | None = None
    name: str | None = None


class MeetingAttendee(BaseModel):
    displayName: str | None = None
    email: str | None = None
    name: str | None = None


class AiFilters(BaseModel):
    # Fireflies returns task as bool, null, or sometimes a string
    task: bool | str | None = False


class Sentence(BaseModel):
    text: str
    speaker_name: str | None = None
    speaker_id: str | int | None = None
    start_time: float | None = None
    end_time: float | None = None
    ai_filters: AiFilters | None = None


class Summary(BaseModel):
    action_items: str | None = None
    overview: str | None = None
    keywords: str | list | None = None
    gist: str | None = None


class Transcript(BaseModel):
    id: str
    title: str | None = None
    date: str | int | None = None  # Unix timestamp (int ms or string)
    organizer_email: str | None = None
    participants: list[str] = []
    meeting_attendees: list[MeetingAttendee] = []
    speakers: list[Speaker] = []
    summary: Summary | None = None
    sentences: list[Sentence] = []
    transcript_url: str | None = None


class WebhookPayload(BaseModel):
    """Fireflies webhook payload. Accepts both camelCase and snake_case field names."""
    meetingId: str | None = None
    meeting_id: str | None = None
    eventType: str | None = None
    event: str | None = None
    clientReferenceId: str | None = None

    @property
    def meeting_id_resolved(self) -> str:
        return self.meetingId or self.meeting_id or ""

    @property
    def event_type_resolved(self) -> str:
        return self.eventType or self.event or ""
