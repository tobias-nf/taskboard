import json
import logging
from pathlib import Path

log = logging.getLogger(__name__)

# Simple file-based store. Replace with a Postgres table if the bridge
# gets its own database later.
_DEFAULT_PATH = Path("/tmp/fireflies-bridge-processed.json")


class ProcessedStore:
    """Track which Fireflies meeting IDs have already been processed."""

    def __init__(self, path: Path = _DEFAULT_PATH):
        self._path = path
        self._ids: set[str] = set()
        self._load()

    def _load(self):
        if self._path.exists():
            try:
                self._ids = set(json.loads(self._path.read_text()))
            except Exception:
                log.warning("Could not load processed store at %s, starting fresh", self._path)
                self._ids = set()

    def _save(self):
        self._path.parent.mkdir(parents=True, exist_ok=True)
        self._path.write_text(json.dumps(sorted(self._ids)))

    def already_processed(self, meeting_id: str) -> bool:
        return meeting_id in self._ids

    def mark_processed(self, meeting_id: str):
        self._ids.add(meeting_id)
        self._save()
        log.info("Marked meeting %s as processed (%d total)", meeting_id, len(self._ids))
