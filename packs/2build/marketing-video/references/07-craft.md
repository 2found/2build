# Stage 7 - Craft (picture and cut)

What makes a directed short look made rather than assembled. These are
standards to judge a cut by, not a recipe: the direction is yours, and every
check behind them WARNs. Read it with [07-direction](07-direction.md) while you
direct and with [07-review](07-review.md) while you look at the draft.

## Motion
Numbers are starting points at 30 fps for a 9:16 short; break one when the
beat asks for it and say why.
- **One lead.** In any half second one thing moves first and the eye follows
  it; the rest waits or follows 0.1–0.2 s behind. A third of the frame or less
  moves at once. Everything moving together reads as noise.
- **Cause before effect.** The hand swings, then the cup falls; the word is
  said, then the number lands. An effect that arrives first reads as a glitch.
- **Nothing living moves linearly.** People, objects and text ease in and out
  (power2 or power3). Linear is for machines, scrolling and the camera's slow
  drift.
- **Anticipation.** A move of any size leans back first: about 8% of the travel
  over the first fifth of its time. The kit adds it to far layer moves and big
  rig swings with no `ease` of their own (`antic`, [07-direction](07-direction.md)).
- **Follow-through and settle.** A move goes slightly past its mark and settles
  back (back.out 1.2–1.6, a few percent). Hair, a sleeve or a prop trails the
  body by 2–4 frames.
- **Hold after landing.** A pose, a number or a word that lands holds still for
  at least 0.4 s (a key moment 1.5 s) before the next move, or nobody reads it.
- **Exits are shorter than entrances,** about two thirds of the time. The
  viewer saw it arrive; it only has to leave.
- **Overshoot by tone.** Comic and kid-facing: big, 10–15%. Calm or serious:
  small, 2–5%, or none. A sad beat does not bounce.
- **Eyes lead.** A character looks where the next thing happens 0.2–0.3 s
  before it happens, and the viewer's eye goes with them.

## Devices
A number or a comparison is a relation, and a relation can be shown. The line
"one in thirty passes" is heard once; 30 dots with one lit is seen and kept.
- Match the device to the relation: a count rising (`counter`), a share
  (`dots`), ranks against ranks (`ladder`), sizes side by side (`bars`), two
  options (`compare`), an order (`steps`), anything else (`html` or a drawn
  `svg` layer).
- The device tells what the picture alone could not. If the plate already
  shows the three cups, three dots add nothing: `device: none`.
- It lands on the word that says it and holds until the line is understood.
  One device per beat; two at once compete.
- Every value it shows is a fact the beat cites.

## Cuts and rhythm
- **Cut for a reason:** a new piece of information, a reaction, a change of
  place, or the answer to a question the last shot raised. A cut with none of
  these resets attention for nothing.
- **Vary the length.** Shots of 1, 3.5, 1.2 and 4 s feel alive; four shots of
  2 s tick like a metronome (V6). Short shots build tension, a long one lets
  something land.
- **Vary the move.** Two pushes in a row read as one long push (V5). Answer a
  push with a hold, a pan or a cut to a close-up.
- **Tempo from the script.** A beat's `tempo` (`fast`, `breath`, `hold`) is the
  writer's note on pace: cut fast beats tight, give a breath beat air, leave a
  hold beat alone.
- **Cut on the action** when a move carries across shots, and on the word
  when the line carries it.

## Key moments
The 2–4 lines marked `key_moment` ([05-script](05-script.md#craft)) are the
ones the viewer should keep. Each gets the picture and the sound at once:
- **Picture:** the clearest framing of the video (a punch-in, a still subject,
  the device landing), on the word, then still for at least 1.5 s after it.
- **Sound:** one hit on the last word, then the bed ducks a further 6 dB over
  the hold, so the line rings in a quiet space. Direction places both; a cue
  of your own within 0.3 s replaces the hit, and `key_sfx: false` drops it.
- Nothing new enters during the hold. A key moment that shares the frame with
  an entrance loses to the entrance.

## Clichés
A frame that could open anyone's video tells the viewer this one is like the
others. The usual ones:
- glowing particles or bokeh over a dark gradient;
- a centred title on a blurred photo;
- a floating phone mock-up turning in space;
- a generic city at night, a starfield or a globe;
- a lens flare or a light leak used as a transition;
- a stock handshake, a lightbulb or a rocket for "an idea" or "growth";
- a scrolling wall of code or numbers;
- text that types itself in for no reason.

**The test: could this frame belong to another video?** Look at each frame of
the contact sheet and ask it. If it could, replace it with something only this
video has: its character doing the thing, its object, its real screen, its
number shown as a device. The cold viewer asks the same question
([07-review](07-review.md#cold-viewer)).
