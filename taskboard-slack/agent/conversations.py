"""Per-user conversation history with sliding window.

Only stores rendered text (user input + assistant response), not tool call
internals. This keeps context small and predictable regardless of how many
tool calls happen per turn.
"""

MAX_MESSAGES = 20  # 10 turns (user + assistant each)


class ConversationStore:
    def __init__(self, max_messages: int = MAX_MESSAGES):
        self._max = max_messages
        self._history: dict[str, list[dict]] = {}  # user_id → messages

    def get(self, user_id: str) -> list[dict]:
        """Return conversation history for a user (may be empty)."""
        return list(self._history.get(user_id, []))

    def add_turn(self, user_id: str, user_text: str, assistant_text: str):
        """Append a user/assistant turn, keeping only the last N messages."""
        if user_id not in self._history:
            self._history[user_id] = []
        h = self._history[user_id]
        h.append({"role": "user", "content": user_text})
        h.append({"role": "assistant", "content": assistant_text})
        # Trim to sliding window
        if len(h) > self._max:
            self._history[user_id] = h[-self._max:]

    def clear(self, user_id: str):
        """Reset conversation for a user."""
        self._history.pop(user_id, None)
