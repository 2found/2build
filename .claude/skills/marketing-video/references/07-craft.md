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

### A blow, a dash, an impact
An effect alone looks like a sticker; the body around it sells the hit.
- **Frame the contact first.** A hit drawn at a tenth of the frame reads as
  nothing at phone size, whatever the effects: punch in (`keys`) to the fist
  and the head on the blow, and make the target react on the contact frame (a
  head snap, a squash, a knockback). Do this before adding anything below.
- **Wind up the opposite way** to the blow, release, then recover slower than
  you struck, the target thrown back (`hits`).
- **Fast in, slow out.** A strike that eases in reads as floaty: power3/expo
  out, a total of well under a second, and a 1–2 frame `flash` at contact.
- **Optional, and unproven:** a `snap` to the release pose, a `freeze` on
  contact (0.08–0.2 s), stepped poses (`steps: 12`) and dash after-images
  (`ghosts`, or `speedlines` linear). In a blind A/B on framed-small fights
  (2026-11-10) none of them gave a visible gain, a freeze delayed the payoff
  and ghosts on a short or vertical move read as a smear. They sell a
  reaction that is already framed; they are not one. Add one only when a
  with/without frame strip shows it helps. A close character's acting is
  different: see [Characters that act](#characters-that-act).
- **Stepped motion needs a matching frame rate.** HyperFrames renders at 30
  fps unless the root sets `data-fps`. Poses held on twos or threes of a
  24 fps count then last 2 frames on some holds and 3 on others, and the
  rhythm wobbles. Set `data-fps="24"` on a cut timed in 24 fps frames.

| You see | Usual fix |
| --- | --- |
| floaty, no punch | faster in, longer hold after, a white frame on contact, shorter total |
| the hit lands out of sync | anchor the freeze, flash and recoil to the one word of the blow |
| weightless | a tighter punch-in on the contact, a bigger target reaction, a bigger wind-up the other way |
| a great effect on a still figure | give the figure a wind-up, a release pose and a recovery |
| mush at phone size | one focal point, bigger shapes, fewer colours |
| washed out glow on paper | glow belongs on dark plates; on paper use ink: lines, solid shapes |
| it pops off | let it shrink, fade or fall away before it ends |

### Characters that act
A drawn character reads as alive through a few habits, whatever the style.
Numbers are frames at 24 fps. The rates, blink and mouth counts come from
animation references (ANN, Animation Obsessive, Bloop, Clip Studio); the
breath and lag figures are estimates to tune by eye.
- **Hold drawings, do not glide.** Hand animation changes the drawing every 2
  frames (12 a second) for normal action and every 3 (8 a second, the TV
  anime default) for calm or weighty action; only a fast release goes to
  every frame. Quantise a character's transforms to those steps (GSAP
  `steps(n)`), and leave the camera and the scenery smooth. A move can start
  on ones and drop to threes as it settles.
- **Pose, hold, pose.** Jump between key poses with little in between and
  hold each one (6–12 frames, longer for a reaction) so it reads.
- **Blinks have a half-shut drawing.** Half 2, shut 1–2, half 2, open: about
  6 frames, every 2–6 s at no fixed interval, one extra after a head turn.
  A blink that only swaps open and shut reads as a flicker.
- **A talking mouth moves.** Three mouths (closed, half, open), a new one
  every 2–3 frames while the voice speaks, never the same twice, closed now
  and then; it shuts 2–3 frames after the sentence ends. A still mouth under
  a voice reads as a dub.
- **Never fully still.** A slow breath (scale 1–2%, a 2.5–3 s cycle, on
  threes), and loose parts (hair, a beard, a tail, a flame, a sleeve)
  swaying on their own cycle 2–4 frames behind the body. Keep both small:
  they are noticed only when missing.
- **A take (surprise, a shock, a hit received).** Squash 2 frames, one
  stretched smear frame toward the move, the pose just past its mark, a
  stepped shake that decays (10, −8, 6, −4, 2 px on twos), a hold, then
  settle on threes. The face changes on the smear, not after. For a big
  one, add radial focus lines boiling on twos while it holds, and at most
  one inverted impact frame (1–2 frames). An impact frame is a flash: one
  per shot, never three in a second.
- On a face filling the frame the owner judged these clearly better than
  the same beats on smooth twos with a one-frame blink and a still mouth
  (stacked side-by-side study, 2026-11-18). That contrasts with the fights
  above, framed small, where stepping showed nothing: the habits sell a
  face and a body the viewer can see.

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
- **A size reference from today is marked as one.** A house cat drawn in
  full colour inside a Cretaceous burrow, to show "cat-sized", read to a
  cold viewer as "cats survived too". Draw the yardstick as a grey ghost
  with a dashed rim and a label, so it is clearly a comparison.
- **One symbol, one meaning per video.** The same cut used "Zzz" for the
  sage's sleep and for a starving dinosaur, and the viewer read the
  starving one as asleep.

## Cuts and rhythm
- **Cut for a reason:** a new piece of information, a reaction, a change of
  place, or the answer to a question the last shot raised. A cut with none of
  these resets attention for nothing.
- **Vary the length.** Shots of 1, 3.5, 1.2 and 4 s feel alive; four shots of
  2 s tick like a metronome (V6). Short shots build tension, a long one lets
  something land.
- **A fresh framing at every cut.** Change at least two of size, angle,
  direction and speed; repeat a framing only on purpose, as a motif.
- **Vary the move.** Two pushes in a row read as one long push (V5). Answer a
  push with a hold, a pan or a cut to a close-up.
- **Tempo from the script.** A beat's `tempo` (`fast`, `breath`, `hold`) is the
  writer's note on pace: cut fast beats tight, give a breath beat air, leave a
  hold beat alone.
- **Cut on the action** when a move carries across shots, and on the word
  when the line carries it.
- **A short changes the picture at least every 2.5 s**: a cut, a punch-in
  or a large pose change. A 43 s cut with nine shots (about 5 s each, one
  frame held 9 s) was "about half as dense as a short needs" to its cold
  viewer, who scored it 4.
- **The peak gets the biggest picture.** The same cut built its voice to the
  asteroid strike and showed a few brown dots, and the viewer named that the
  moment they would leave. Give the story's peak the frame's full width,
  a flash and a shake; spend the drawing time there first.

## Empty space
A frame with room left over is a framing problem, not a decoration problem.
- **The subject takes the room.** Bring it closer, make it larger or move it
  into the gap (a punch-in, the object the line names drawn big, the next
  thing the voice needs). Scenery added to cover the gap leaves the subject
  as small as it was.
- **A band that never changes is empty too.** One cut filled the lower 40% of
  every frame with the same lantern pond for three minutes after its cold
  viewer named the gap: the frame measured full and the viewer still had
  nothing to look at there. A place earns its area by changing with the
  story or by being where the action happens.
- Room kept on purpose (a breath beat, a held key moment, space the captions
  need) is a choice: say which.

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

Generated design also drifts to the same few choices, and viewers now spot
them: a purple-to-cyan gradient, warm cream with terracotta or indigo, slate
navy, glass cards floating on a neutral ground, neon glow or bloom on
everything, bouncy elastic easing on a serious subject, spotless flat
surfaces, a camera that drifts for no reason, and Inter, Roboto, Montserrat,
Poppins or Space Grotesk as the type. Any of them can be right when this
video's idea asks for it; then say why.

**The test: could this frame belong to another video?** Look at each frame of
the contact sheet and ask it. If it could, replace it with something only this
video has: its character doing the thing, its object, its real screen, its
number shown as a device. The cold viewer asks the same question
([07-review](07-review.md#cold-viewer)).
