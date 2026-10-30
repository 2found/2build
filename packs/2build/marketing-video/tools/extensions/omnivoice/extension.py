"""OmniVoice (k2-fsa/OmniVoice, open weights) as an mvf voice engine: clone a reference
voice or design a new one, voiced locally and heard back by Whisper.

No voices ship with it. A clone reference is yours: <campaign>/voices/<name>.wav + .txt
(the words it says), or .babysit/marketing-video/voices/ for every campaign on this
machine — or design one with `mvf voice-design` and pick a candidate.

The worker needs torch, OmniVoice and Whisper. By default it runs in an ephemeral uv
environment (first run downloads ~2 GB of wheels and the model weights from Hugging
Face); set MVF_OMNIVOICE_PYTHON to a Python that already has them to skip that.
"""
from __future__ import annotations

import os
from pathlib import Path

from mvflib.extensions import Voice

HERE = Path(__file__).resolve().parent
DEPS = ("omnivoice", "torch", "torchaudio", "torchcodec", "transformers", "num2words", "regex", "numpy")


def _command() -> list[str]:
    py = os.environ.get("MVF_OMNIVOICE_PYTHON", "").strip()
    if py:
        return [py, str(HERE / "tts_omnivoice.py")]
    return ["uv", "run", "--no-project", "--python", "3.12", *[x for d in DEPS for x in ("--with", d)],
            "python", str(HERE / "tts_omnivoice.py")]


VOICES = {
    "omnivoice": Voice(
        "OmniVoice local TTS (clone a reference or design a voice) + Whisper hearing check; needs uv or $MVF_OMNIVOICE_PYTHON",
        modes=("take", "design", "rehear"),
        command=_command(),
        opts={"retries": "re-voice a cue that dropped/added words, up to N times (default 2)",
              "asr-model": "Whisper model for the hearing check (default openai/whisper-small)"},
    ),
}
