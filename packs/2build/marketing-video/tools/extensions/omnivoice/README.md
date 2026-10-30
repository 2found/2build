# omnivoice — local voice engine for mvf

`mvf voice --engine omnivoice` (or `"voice_engine": "omnivoice"` in campaign.json)
voices a script with [OmniVoice](https://huggingface.co/k2-fsa/OmniVoice) on
this machine. Whisper then hears each line back. A line that dropped or added
words is voiced again with a new seed.

- **Voices.** None ship. Voice references are machine-local and never committed.
  Put `<name>.wav` plus `<name>.txt` (the exact words the clip says, 10–15 s) in
  `<campaign>/voices/` or in `.babysit/marketing-video/voices/`. Then set the
  script to `voice: {mode: "clone", ref: "<name>"}`. To create a voice, run
  `mvf voice-design --name N --instruct "…" --sample "…"`, listen to the
  candidates, then `mvf voice-pick N.candK --as N`.
- **Only clone people who said yes.** Clone your own voice or a narrator who
  consented to this use. Never clone a public figure.
- **Environment.** By default the worker runs in an ephemeral `uv run --with …`
  env. The first run downloads torch and the model weights. To reuse an
  environment that already has `omnivoice torch torchaudio transformers
  num2words regex numpy`, set `MVF_OMNIVOICE_PYTHON=/path/to/python`.
- **Heavy jobs.** If the machine is shared, set `MVF_HEAVY` or `"heavy"` in
  `.babysit/marketing-video/config.json` to a queue or lock command. Every
  model and render job then runs through that prefix.
- **Options.** `--opt retries=3` and `--opt asr-model=openai/whisper-medium`.

`tts_text.py` (cleaning and chunking) and `vi_numbers.py` (Vietnamese numbers
read as words) are stdlib-only helpers for the worker. mvf's tests exercise
them on bare python3.
