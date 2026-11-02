"""Echo transcriber — LiveKit Agents worker with a custom Vosk STT plugin.

Joins every room assigned by LiveKit, transcribes French speech locally with
Vosk (kaldi-fr), and broadcasts caption/final-transcript messages on the room
data channel so the Svelte client can render live captions without any extra
plumbing. Every FINAL utterance is also POSTed to the Echo API, which appends
it to the open call's transcript.

Run locally:
    pip install -r requirements.txt
    python agent.py download-files          # pulls the small-fr model
    LIVEKIT_URL=ws://localhost:7880 \
    LIVEKIT_API_KEY=... LIVEKIT_API_SECRET=... \
    ECHO_API_URL=http://localhost:4020 TRANSCRIBER_TOKEN=... python agent.py start
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
from dataclasses import dataclass
from urllib.parse import quote

import aiohttp
from livekit import rtc
from livekit.agents import AgentSession, JobContext, WorkerOptions, cli, stt
from livekit.agents.types import DEFAULT_API_CONNECT_OPTIONS, NOT_GIVEN
from vosk import Model, KaldiRecognizer, SetLogLevel

VOSK_MODEL_PATH = os.environ.get("VOSK_MODEL_PATH", "models/vosk-model-small-fr-0.22")
SAMPLE_RATE = 16_000

ECHO_API_URL = os.environ.get("ECHO_API_URL", "").rstrip("/")
TRANSCRIBER_TOKEN = os.environ.get("TRANSCRIBER_TOKEN", "")
PERSIST_TIMEOUT_SECONDS = 3.0

logger = logging.getLogger("echo.transcriber")

PERSIST_TRANSCRIPTS = bool(ECHO_API_URL and TRANSCRIBER_TOKEN)
if not PERSIST_TRANSCRIPTS:
    logger.warning(
        "ECHO_API_URL or TRANSCRIBER_TOKEN is unset: live captions still broadcast, "
        "but transcripts are not persisted"
    )


_INFLIGHT: set[asyncio.Task] = set()


def _spawn(coro, what: str) -> None:
    """Run `coro` in the background, holding a strong reference until it ends.

    `asyncio.ensure_future` alone is not enough: the loop keeps only a weak
    reference, so CPython may collect a task mid-flight and drop the utterance
    with no log line. The done-callback also surfaces the exception, which an
    unawaited task would otherwise hide until interpreter shutdown.
    """
    task = asyncio.ensure_future(coro)
    _INFLIGHT.add(task)
    task.add_done_callback(lambda done: _on_task_done(done, what))


def _on_task_done(task: asyncio.Task, what: str) -> None:
    _INFLIGHT.discard(task)
    if task.cancelled():
        return
    error = task.exception()
    if error is not None:
        logger.warning("%s failed: %r", what, error)


async def _drain_inflight() -> None:
    pending = [task for task in _INFLIGHT if not task.done()]
    if pending:
        await asyncio.wait(pending, timeout=PERSIST_TIMEOUT_SECONDS + 1.0)


@dataclass
class CaptionMessage:
    type: str  # "caption" | "transcript"
    speaker: str
    text: str
    final: bool

    def encode(self) -> bytes:
        return json.dumps(self.__dict__, ensure_ascii=False).encode("utf-8")


class VoskSTT(stt.STT):
    """Streaming STT that feeds 16 kHz mono PCM frames into a KaldiRecognizer."""

    def __init__(self) -> None:
        super().__init__(capabilities=stt.STTCapabilities(streaming=True, interim_results=False))
        SetLogLevel(-1)
        self._model = Model(VOSK_MODEL_PATH)

    async def _recognize_impl(self, buffer, language):  # pragma: no cover - batch path unused
        raise NotImplementedError("VoskSTT is streaming-only")

    def stream(self, *, language=NOT_GIVEN, conn_options=DEFAULT_API_CONNECT_OPTIONS):
        return VoskStream(self, conn_options)


class VoskStream(stt.SpeechStream):
    def __init__(self, stt_instance: VoskSTT, conn_options=DEFAULT_API_CONNECT_OPTIONS) -> None:
        super().__init__(stt=stt_instance, conn_options=conn_options)
        self._recognizer = KaldiRecognizer(stt_instance._model, SAMPLE_RATE)
        self._recognizer.SetWords(False)

    async def _run(self) -> None:
        loop = asyncio.get_running_loop()

        def recognize(frame: bytes) -> dict | None:
            if self._recognizer.AcceptWaveform(frame):
                return json.loads(self._recognizer.Result())
            return None

        async for frame in self._input_ch:
            if isinstance(frame, stt.SpeechStream._FlushSentinel):  # end of utterance
                text = json.loads(self._recognizer.FinalResult()).get("text", "")
                if text:
                    self._send_final(text)
                continue
            pcm = getattr(frame, "data", b"")
            if not pcm:
                continue
            result = await loop.run_in_executor(None, recognize, bytes(pcm))
            if result and result.get("text"):
                self._send_final(result["text"])

    def _send_final(self, text: str) -> None:
        self._event_ch.send_nowait(
            stt.SpeechEvent(
                type=stt.SpeechEventType.FINAL_TRANSCRIPT,
                alternatives=[stt.SpeechData(language="fr-FR", text=text)],
            )
        )


def _speaker_of(event) -> str:
    # Attribute naming varies across livekit-agents versions; fall back to
    # "unknown" rather than crash on a rename.
    speaker = getattr(event, "participant", None) or getattr(event, "speaker_id", None) or "unknown"
    if hasattr(speaker, "identity"):
        return str(speaker.identity)
    return str(speaker)


async def _post_transcript(http: aiohttp.ClientSession, room_name: str, speaker: str, text: str) -> None:
    """POST one final utterance to the Echo API, swallowing every failure.

    Persistence is best effort by design: a dead API, a slow API or a room
    with no open call must never interrupt the live caption stream.
    """
    url = f"{ECHO_API_URL}/api/rooms/{quote(room_name, safe='')}/transcript"
    try:
        async with http.post(
            url,
            json={"speaker": speaker, "text": text},
            headers={"Authorization": f"Bearer {TRANSCRIBER_TOKEN}"},
            timeout=aiohttp.ClientTimeout(total=PERSIST_TIMEOUT_SECONDS),
        ) as response:
            if response.status == 404:
                logger.debug("no open call for room %s, utterance dropped", room_name)
            elif response.status != 204:
                logger.warning("transcript refused for room %s: HTTP %s", room_name, response.status)
    except asyncio.CancelledError:
        raise
    except Exception as error:
        logger.warning("transcript post failed for room %s: %r", room_name, error)


async def entrypoint(ctx: JobContext) -> None:
    await ctx.connect()

    room: rtc.Room = ctx.room
    local = room.local_participant
    session = AgentSession(stt=VoskSTT())

    http: aiohttp.ClientSession | None = None
    if PERSIST_TRANSCRIPTS:
        http = aiohttp.ClientSession()
        session_http = http

        async def _close_http() -> None:
            await _drain_inflight()
            await session_http.close()

        ctx.add_shutdown_callback(_close_http)

    @session.on("user_input_transcribed")
    def _on_transcript(event) -> None:
        speaker = _speaker_of(event)
        message = CaptionMessage(
            type="caption",
            speaker=speaker,
            text=event.transcript,
            final=event.is_final,
        )
        _spawn(local.publish_data(message.encode(), topic="transcription"), "caption publish")
        if http is not None and event.is_final and event.transcript:
            _spawn(
                _post_transcript(http, room.name, speaker, event.transcript),
                "transcript persist",
            )

    await session.start(room=room)


if __name__ == "__main__":
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint))
