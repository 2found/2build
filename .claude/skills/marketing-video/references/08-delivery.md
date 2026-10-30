# Stage 8 - Delivery

Output: `08-delivery/<sid>/<hook>/<platform>/` with the video, `captions.srt`,
`caption.txt` (TikTok) or `post.txt` plus thread (X), `cover.jpg` (frame 0),
`contact.jpg`, `qa.md`.

Do not deliver (draft only) while placeholders remain, timing is estimated, or
the audio is from an older script than the picture.

## Platform specs (re-read the source before trusting numbers older than 90 days)
| | TikTok | X |
| --- | --- | --- |
| Canvas | 1080x1920, 9:16 | 1080x1920 web upload; 720x1280 for API upload |
| Length | hard 3 s-10 min; sweet 15-60 s | hard 0.5-140 s (no Premium); sweet 10-45 s |
| File | up to about 287 MB on a phone | up to 512 MB, MP4/MOV H.264 + AAC |
| Sound default | on | off (autoplay muted) |
| Copy | caption up to 4000 chars, 3-5 hashtags | 280 weighted chars, 0-2 hashtags |
Sources: TikTok ads creative best practices; X media best practices (docs.x.com).

## Steps
1. Render at delivery quality.
2. Master audio to about -14 LUFS integrated, true peak <= -1.5 dBFS (ffmpeg
   `loudnorm`).
3. Encode per platform: H.264 High, yuv420p, 30 fps, CRF about 18 with a
   maxrate of 10-12 Mb/s (X API: 720x1280 about 6 Mb/s), AAC LC stereo 48 kHz
   192 kb/s, `+faststart`. Never upscale.
4. QA each file with `ffprobe`/ffmpeg and write `qa.md`: codec, pix_fmt, fps,
   size and aspect correct; audio AAC LC, at most 2 channels; duration inside
   platform limits and within 0.2 s of the storyboard; file size under the limit;
   loudness within 1.5 LU of target and peak under ceiling; frame 0 not black; no
   freeze over 2.5 s; no silence inside the voice span.
5. Cut `captions.srt` from the measured word times, `cover.jpg` from frame 0,
   `contact.jpg` and 5 spot frames.

## Before calling it done
Numbers are not ears. Watch each delivered file once with sound and once muted
(for X): the hook reads at 0 s, captions never cover the subject, the real
screen is legible, the voice is clear over the bed, the CTA matches its fact.
Look at the contact sheet for dead stretches. Mark `qa.md` `PENDING LISTEN` until
you or the user have done this; you cannot hear it yourself.

## Hand-off (the user posts)
Report per file: path, duration, size, LUFS, QA verdict, what you watched, open
limits (unverified trends, QA-build screens, unknown handles). Posting notes:
TikTok: add the chosen commercial-library or trend sound in-app at a low level,
caption from `caption.txt`, cover from `cover.jpg`. X: upload the file, post
`post.txt`, reply with the thread and the link. Post A/B hooks as separate posts
on different days or as ad-group creatives. Nothing is uploaded by this flow.
