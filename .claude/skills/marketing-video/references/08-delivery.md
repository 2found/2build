# Stage 8 - Delivery

Output: `08-delivery/<sid>/<canvas>/<platform>/`, holding:
- the video, `captions.srt`, `cover.jpg` (frame 0), `contact.jpg`, the spot
  frames and `qa.json` (or `qa.md` by hand);
- the platform's copy: `caption.txt` (TikTok), `post.txt` plus the thread
  (X), or `title.txt`, `description.txt` and `tags.txt` (YouTube);
- `thumbnail.jpg` for YouTube long-form.

## Bar
- **Nothing ships as a draft.** Do not deliver while placeholders remain,
  timing is estimated, the audio is older than the picture, or the script
  shows unapproved art.
- **Platform specs.** Re-read the source of any number older than 90 days.

| | TikTok | X | YouTube Shorts | YouTube long-form |
| --- | --- | --- | --- | --- |
| Canvas | 1080x1920, 9:16 | 1080x1920 web upload; 720x1280 for API upload | 1080x1920, 9:16 (1:1 allowed) | 1920x1080, 16:9 |
| Length | hard 3 s-10 min; sweet 15-60 s | hard 0.5-140 s (no Premium); sweet 10-45 s | hard 3 min; sweet 15-60 s | 15 min until the channel is verified; sweet 1-8 min |
| File | up to about 287 MB on a phone | up to 512 MB, MP4/MOV H.264 + AAC | H.264 High, closed GOP of half the frame rate, AAC-LC 48 kHz | same; thumbnail JPG/PNG 1280x720+, <= 2 MB |
| Sound default | on | off (autoplay muted) | on | on |
| Copy | caption up to 4000 chars, 3-5 hashtags | 280 weighted chars, 0-2 hashtags | title <= 100, description <= 5000, tags <= 500 chars; first 3 hashtags show, over 60 drops all | same, plus chapters |

| | Instagram Reels | Facebook Reels | LinkedIn video |
| --- | --- | --- | --- |
| Canvas | 1080x1920, 9:16 (min 720 px) | 1080x1920, 9:16 | 1080x1920 here (it takes 1:2.4 to 2.4:1) |
| Length | hard 3 s-15 min; sweet 15-90 s; only reels up to 3 min reach non-followers | no organic limit published; sweet 15-90 s | hard 3 s-10 min (Pages); sweet 15-90 s |
| File | up to 300 MB, H.264, closed GOP, AAC <= 48 kHz | H.264, closed GOP 2-5 s, AAC | up to 5 GB, MP4 |
| Sound default | on | on | off (autoplay muted) |
| Copy | caption up to 2200 chars, at most 5 hashtags; links not clickable | about two lines show before "more"; 0-3 hashtags | post up to 3000 chars; links clickable |

Sources: TikTok ads creative best practices; X media best practices
(docs.x.com); YouTube Help 10059070 (Shorts length), 1722171 (upload
encoding), 6390658 (hashtags) and 72431 (thumbnails); Instagram Graph API media
reference, Meta's Reels publishing guide, LinkedIn Help a548372 and a1311816.
`tools/mvflib/platforms.json` carries the URLs.

- **Encoding.** Every file is H.264 High, yuv420p, at most 60 fps, inside the
  platform's size and aspect (1:3 to 3:1). Audio is AAC LC, mono or stereo.
  Never upscale.
- **Loudness.** Master to -14 LUFS integrated with static gain and a true-peak
  limiter, so the sample peak stays under -1 dBFS after AAC. Do not use a
  dynamic normaliser, which undoes the voice-to-bed balance.
- **QA per file** (`mvf qa`, or ffprobe by hand). It FAILs:
  - a wrong codec, pix_fmt or audio codec;
  - over 60 fps, oversize or an off aspect;
  - no audio, or more than 2 channels;
  - a duration outside the platform's limits or more than 0.2 s off the
    timeline;
  - a file over the size limit;
  - a black frame 0.

  It WARNs on loudness more than 1.5 LU off, a hot peak, a freeze over
  2.5 s, silence inside the voice, and a canvas the player letterboxes.
- **Thumbnail** (long-form). It FAILs if it is not the canvas, smaller than
  1280x720, over 2 MB, or black.
- **Captions** come from the measured word times. The cover is frame 0.
- **Watched and heard.** Watch each file once with sound and once muted. Check
  that:
  - the hook reads at 0 s;
  - captions never cover the subject;
  - the real screen is legible;
  - the voice is clear;
  - the CTA matches its fact.

  Keep `PENDING LISTEN` in the QA file until you or the user have heard it.

## AI labels
`delivery.json` carries `ai_disclosure`, and export notes it per platform.
- TikTok and YouTube require the label on realistic generated or altered
  picture, sound or video. That means photoreal art, and on TikTok a synthetic
  narrator (a clone, designed or preset voice).
- Illustration, texture, thumbnails and caption help do not need it on
  YouTube. On TikTok it is recommended for any generated art.
- Where the label says ON, the user sets it at upload: TikTok "AI-generated
  content", YouTube Studio "Altered or synthetic content".
- X has no toggle. Say it in the post when the synthetic part could mislead.

Never hide a required label.

## Tool
- `mvf export <proj>` renders, masters, encodes and QAs for the script's
  platforms on that project's canvas (`--platforms` narrows it, `--draft`
  makes a rough cut). It writes the folder above and `delivery.json`.
- `mvf qa <video> --platform P --proj D` re-measures one file.
- Outside HyperFrames, do the same steps by hand and write `qa.md`.

## Hand-off (the user posts)
Report each file with its path, duration, size, LUFS, QA verdict, what you
watched, and the open limits (unverified trends, placeholder screens, unknown
handles). Posting notes:
- **TikTok:** add the chosen library or trend sound in the app at a low level.
  Use `caption.txt` and `cover.jpg`.
- **X:** upload the file, post `post.txt`, then reply with the thread and the
  link.
- **YouTube:** use `title.txt`, `description.txt` and `tags.txt`. A link in a
  Shorts description does not click, so pin a comment. Long-form also gets
  `thumbnail.jpg`, the chapters, and the AI label when `ai_disclosure` says
  so.

Nothing is uploaded by this flow.
