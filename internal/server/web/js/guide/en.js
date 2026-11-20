// The guide in English. The Russian one is in ru.js, with the same sections in
// the same order: a test in internal/server compares their ids.

export default {
  title: 'Guide',
  lede: `Wheel alignment is neither magic nor a workshop secret. It is four wheels, a few angles and the order in which
    to set them. Here is everything you need to do it yourself and do it right, from installing the program to the
    printed report.`,
  sections: [

['start', 'In short: five steps', `
<ol>
  <li><b>Vehicle.</b> Find your car in the database or take the guidance for its class. Check the front and rear
    suspension type — the checks and the adjustment advice depend on it.</li>
  <li><b>Preparation.</b> A checklist for your suspension: pressures, play, load, floor. Adjusting a worn suspension is
    pointless — the angles will wander off at the first bump.</li>
  <li><b>Measurement.</b> Choose what to measure with: strings and an inclinometer, a phone, a camera or a sensor of your
    own. Methods can be combined — for example camber and caster by phone, toe by string.</li>
  <li><b>Adjustment.</b> A screen like an alignment bench: big figures, green when in spec, red when not; above each, a
    tolerance scale with a pointer. Tap an angle and a panel shows what adjusts it on your car. Before the first turn
    of a spanner, take the “before” snapshot (F5).</li>
  <li><b>Report.</b> When everything is green, take the “after” snapshot (F5) and open the report with its before/after
    table (F6). It can be printed or saved as PDF.</li>
</ol>
<p><b>Nothing gets lost.</b> The chosen vehicle, the figures you typed, the “before” and “after” snapshots, the checklist and
  the plate number are kept by the program: close it, open it tomorrow — everything is still there, and the screen opens
  where you stopped. Start the next car with <b>“New vehicle”</b> in the menu on the left (or in the report): everything
  of the previous one is cleared, while phone and camera calibrations stay.</p>
<p><b>The “what next” line</b> on the adjustment screen leads you through the whole job: the “before” snapshot, then the
  angles in the right order, the “after” snapshot, the report. Its button does the next thing right away.</p>
<div class="note">New to this? Start the demonstration (F9 on the adjustment screen) and press F10 — the program shows by
  itself in which order and how the angles are set. With the ← → keys you can “turn” any angle yourself and watch what
  happens to the others.</div>`],

['install', 'Installing and first start', `
<ol>
  <li><b>Download</b> <code>wheelalign.exe</code> from the project's releases page on GitHub. No installation is needed: it
    is a single file and can even live on a USB stick.</li>
  <li><b>Run it.</b> The program opens its own window, powered by WebView2, which is built into Windows 10 and 11. If the
    window cannot open, the program opens in your ordinary browser and tells you what to install.</li>
  <li><b>Language</b> — the RU / EN switch in the top right corner. On first start the system language is used; your
    choice is remembered.</li>
  <li><b>No internet needed.</b> Everything runs on this computer. Only the phone sensor needs a network, and a local
    one (Wi-Fi) at that, not the internet.</li>
</ol>
<p>Where your data lives: <code>%APPDATA%\\wheelalign</code> (on Linux <code>~/.config/wheelalign</code>). It holds the settings
  (<code>settings.json</code>), the current job — the vehicle on the stand, the figures typed in, the snapshots
  (<code>session.json</code>), your own specifications (the <code>vehicles</code> folder), phone calibrations
  (<code>phones.json</code>) and phone access (<code>phone-link.json</code>), the phone certificate (<code>tls</code>) and the log
  <code>wheelalign.log</code> — attach it when reporting a bug.</p>
<p class="muted">Start options: <code>-browser</code> — open in the browser instead of a window; <code>-data &lt;folder&gt;</code> —
  keep the data elsewhere (for example next to the program on a USB stick). Full list: <code>wheelalign -h</code>.</p>`],

['access', 'Settings and accessibility', `
<p>The program should suit everyone: whoever works in a bright garage, sees poorly or cannot tell colours apart, lies
  under the car alone or uses only a keyboard. All of it is in “Settings” (the ⚙ icon at the top right): it applies at
  once and is kept in the program's settings file.</p>
<ul>
  <li><b>Theme.</b> Dark — like an aligner's screen; light — for a bright garage and sunlight; high contrast — black and
    white, bright colours and thick borders.</li>
  <li><b>Interface size</b> — normal, large, extra large: everything grows at once — text, buttons, figures.</li>
  <li><b>Status colours.</b> If red and green are hard to tell apart, choose “Blue and orange”. A sign always stands next
    to the colour: <b>✓</b> — in tolerance, <b>!</b> — near the limit, <b>✕</b> — out of tolerance; the signs show on a
    black-and-white printout of the report too.</li>
  <li><b>No animation</b> — for those bothered by blinking. By default the program follows the Windows “Show
    animations” setting.</li>
  <li><b>Adjusting by ear.</b> Choose an angle on the adjustment screen and turn the sound on (<kbd>F7</kbd>): out of
    tolerance the program beeps — faster the closer it gets — a chime sounds on entering the tolerance and a low tone on
    leaving it. The voice (<kbd>F8</kbd>) reads the chosen angle aloud and says when it is in tolerance and when
    everything is. The voice is the system one, from Windows; if there is none for your language: Settings → Time &amp;
    language → Speech → “Add voices”.</li>
  <li><b>No mouse needed.</b> Everything works from the keyboard: <kbd>Tab</kbd> — to the next element, <kbd>Enter</kbd>
    or Space — press, <kbd>Esc</kbd> — close the panel; the F-keys are labelled at the bottom of the screen. A frame
    shows where the focus is. A screen reader (the one built into Windows, or NVDA) reads the readings, the result and
    the program's messages.</li>
  <li><b>Workshop.</b> Its name, address and the person doing the job go into the report header.</li>
  <li><b>The welcome</b> on first start walks through these settings and helps choose a way to measure. To open it
    again — “Settings → Show the welcome again”.</li>
</ul>`],

['angles', 'The angles in plain words', `
<table class="params"><thead><tr><th>Angle</th><th>What it is</th><th>When it is off</th></tr></thead><tbody>
  <tr><td><b>Camber</b></td><td>Tilt of the wheel across the car, seen from the front. “Plus” — the top of the wheel leans out.</td>
    <td>One edge of the tread wears (the outer one with positive, the inner with negative camber). A left–right
    difference above ~0.5° pulls the car towards the side with more camber.</td></tr>
  <tr><td><b>Toe</b></td><td>Rotation of the wheel seen from above. “Plus” — the front edge of the wheel is closer to the
    car's centreline (toe-in).</td>
    <td>The most expensive angle: it feathers a tyre in a few thousand kilometres. A crooked steering wheel when driving
    straight means the toe is split wrongly between the wheels.</td></tr>
  <tr><td><b>Caster</b></td><td>Fore–aft tilt of the steering axis (the kingpin on kingpin axles). “Plus” — the top of
    the axis leans back.</td><td>Steering does not return to centre, the car wanders at speed. A left–right difference
    pulls towards the side with less caster.</td></tr>
  <tr><td><b>Steering axis inclination (SAI)</b></td><td>Tilt of the steering axis across the car. Fixed by design.</td>
    <td>Not adjustable. A left–right difference points to a bent strut, knuckle or axle beam.</td></tr>
  <tr><td><b>Included angle</b></td><td>SAI + camber.</td><td>Differs left to right by more than 1° — a part is bent;
    camber adjustment will not cure it.</td></tr>
  <tr><td><b>Thrust angle</b></td><td>Where the rear axle points relative to the car's centreline: half the difference of
    the rear wheels' toe.</td><td>The car “crabs”, the steering wheel is off-centre when driving straight although the
    front toe is in spec.</td></tr>
</tbody></table>
<div data-figs="camber toe caster"></div>
<div data-figs="sai thrust"></div>
<p>In short, what pulls a car (Hyundai bulletin 20-SS-001H): it pulls towards the wheel with more positive camber — the
  front camber difference should be under 0.5° — and towards the wheel with less caster. Toe does not cause a pull, but
  tyre wear and whether the steering wheel sits straight depend on it. Tyres (unequal pressure, ply steer and conicity),
  a dragging brake, a sagging spring, road crown and crosswind can pull a car too.</p>`],

['vehicle', 'Step 1. Vehicle', `
<ul>
  <li><b>Search</b> by make and model, in Latin or Cyrillic letters: “Volga 3110”, “GAZelle”, “2107”, “Hilux”. The filters
    at the top narrow the list to cars, SUVs, vans, trucks or buses; the “Year” box drops other generations.</li>
  <li><b>Not sure of the spelling?</b> While the search box is empty, every make is listed below by region (USSR and
    Russia, Europe, USA, Asia) with its number of models, along with recently chosen vehicles. Click a make to keep only
    its models.</li>
  <li>Every entry carries a <b>source badge</b>: factory manual, community, unverified, design only. The weaker the badge,
    the more carefully treat the figures — see “Where the specifications come from”.</li>
  <li><b>Model not in the database?</b> Choose the guidance for its class (“car with MacPherson struts”, “truck with a
    kingpin beam axle”…). These are typical values for the design, not for your model: the program shows how far the
    angles are from typical and says so honestly on every screen.</li>
  <li><b>Suspension.</b> The program fills in the suspension type from the database; if it is missing, choose it
    yourself. “How do I tell which one I have?” explains how to tell a kingpin from a ball joint, MacPherson from double
    wishbones.</li>
  <li><b>Dimensions.</b> The rim diameter converts toe from degrees to millimetres (many manuals give it that way). The
    front and rear track widths are used to check the strings.</li>
  <li><b>Your own specifications.</b> If you have a workshop manual, use “Enter specs from the manual”. The program checks
    the figures for typos and uses them on this computer.</li>
</ul>
<p class="muted">You can also work without choosing a vehicle — the angles are then shown without an in-spec / out-of-spec verdict.</p>`],

['prep', 'Step 2. Preparation', `
<p>Items with a picture have a <b>“What it looks like”</b> button — the diagram opens right under the item.</p>
<p>The checklist follows the chosen suspension: kingpin play for a beam axle, strut top bearings for MacPherson, U-bolts
  and the centre bolt for leaf springs. The items are grouped — safety, tools, floor, vehicle, front and rear suspension —
  and each group has its own counter. In large type: what to do; in small type: why. Tick items off as you go — the ticks
  are saved; once everything is ticked, “All done — ready to measure” appears.</p>
<p>The “Tools” group depends on the way of measuring: a line and an inclinometer for strings, a bar and a shared Wi-Fi
  network for a phone, targets and a tripod for a camera. You can choose the way right in this group — the
  “Measurement” step will already have it selected.</p>
<ul>
  <li><b>Tyre pressures</b> — as on the door-pillar placard, with cold tyres. A 0.3 bar left–right difference already
    changes camber.</li>
  <li><b>Play</b> — tie-rod ends, ball joints or kingpins, wheel bearings, bushings. If a wheel rocks by hand, repair first.</li>
  <li><b>Load</b> — as the manual requires: usually a full tank, spare wheel and tools in place, nobody inside. For trucks
    and buses — the stated axle load.</li>
  <li><b>Steering straight.</b> Before measuring toe, set the steering wheel straight and lock it (a prop between wheel
    and seat).</li>
  <li><b>Let the suspension settle.</b> After lifting the car, bounce the body and roll it 3–5 metres forward — the arms
    return to their working position. Do not roll it back or push it sideways: the bushings would stay loaded differently
    from how they are when driving.</li>
</ul>`],

['tools', 'Tools and floor', `
<div data-figs="level settle rideHeight"></div>
<div data-figs="play dryPark"></div>
<div data-figs="turnplates pedal wheelLock"></div>
<table class="params"><thead><tr><th>What</th><th>What for</th><th>Substitute</th></tr></thead><tbody>
  <tr><td>Fishing line 0.3–0.5 mm, 2 × 5 m</td><td>Reference lines for toe</td><td>Not thread — it sags and stretches</td></tr>
  <tr><td>4 stands up to hub height</td><td>Hold the strings</td><td>Bricks with blocks, buckets of sand</td></tr>
  <tr><td>Weights of 2–4 kg</td><td>Tension the strings without sag</td><td>Bottles of water</td></tr>
  <tr><td>Tape measure, calipers or a rule</td><td>Distances to the rim</td><td>—</td></tr>
  <tr><td>Inclinometer or smartphone</td><td>Camber</td><td>Spirit level with a protractor</td></tr>
  <tr><td>Straight bar 40–50 cm</td><td>Holds the inclinometer or phone against the rim flanges</td><td>Aluminium straightedge</td></tr>
  <tr><td>Turn plates</td><td>Caster</td><td>Two sheets of tin with grease between them</td></tr>
  <tr><td>Axle stands</td><td>Safety</td><td><b>Never a jack instead. Never.</b></td></tr>
</tbody></table>
<p style="margin-top:12px"><b>The floor matters most.</b> It need not be “level” but it must be <i>flat</i>: all four wheels in
  one plane. A 5 mm dip under one wheel on a 1400 mm track is 0.2° of camber error, half a factory tolerance. Check with
  a water level (a clear hose filled with water): the contact patches should differ by less than 2 mm. Shim with steel
  plates, not wood. Turn plates and the packing under the rear wheels must be the same height.</p>`],

['string', 'Measuring: strings and an inclinometer', `
<ol>
  <li><b>Strings</b> run along both sides, outside the wheels, at hub-centre height, tensioned by weights. The distance from
    the string to the hub centre at the front and at the rear <b>need not be equal</b>: with different front and rear
    track widths the difference must be (rear track − front track) / 2. Enter the four distances in “String setup
    check” and the program tells you which end to move and by how much.</li>
  <li><b>Camber</b> — the inclinometer on a bar held against the rim flanges (not the tyre). “Plus” — the top of the wheel
    leans out. If your inclinometer reads the other way, tick the box. Then roll the car half a wheel turn (a chalk mark
    on the tyre) and measure the same spot of the rim again. The program averages the two readings and removes rim
    runout; runout above 0.5° means a bent rim and unreliable readings for that wheel.</li>
  <li><b>Toe</b> — from the string to the rim flange at the front and rear of the wheel, strictly at hub height. Higher or
    lower mixes camber into the reading. A wheel pointing inwards has the larger front distance; enter it as it is.</li>
  <li><b>Caster</b> — on turn plates: camber with the wheel steered 20° out and 20° in. “Out” for the left wheel is the
    steering wheel turned left, for the right wheel — turned right. The program solves this exactly, not with the
    “swing × 1.5” rule, which is off by up to 0.27° on cars with large caster. Read the steer angle off the turn plate
    scale or chalk it on the floor.</li>
</ol>
<p>Every figure you enter goes straight to the adjustment screen and is kept. While working: turn the tie rod, measure,
  enter the new figure. The screen updates at once. Delete a figure and it leaves the screen too; “Clear all fields”
  removes everything typed by hand at once.</p>
<div data-figs="bar strings runout"></div>
<div data-figs="steer:left steer:right"></div>`],

['phone', 'Measuring: a phone on the wheel', `
<ol>
  <li>Press <b>“Phone”</b> at the top right — from any screen — and scan the QR code with the phone's camera. Wi-Fi access
    switches on by itself. The same is on “Measurement → Phone on the wheel”. Phone and computer must be on the same
    Wi-Fi network (a hotspot on the phone itself will do).</li>
  <li>The phone's browser warns about the certificate — it was made by this program, not by a certificate authority.
    “Advanced” → “Proceed to the site”. Windows may ask for firewall permission — allow private networks.</li>
  <li>On the phone, tap the wheel — the buttons sit where the wheels are on the car — and press “Start sensors”. On an
    iPhone Safari asks for motion sensor access — allow it.</li>
  <li>At the top there are three tabs — what you measure: <b>Camber</b>, <b>Caster</b>, <b>Toe</b>.</li>
  <li><b>Camber → “Preparing the phone”</b>, once: <b>1. Phone calibration</b> — on a table screen up, then screen down;
    this removes the accelerometer's own error, which on phones can reach 1–2°. <b>2. Mount calibration</b> — the phone
    on the bar on the rim, then upside down on the same spot; this removes the tilt caused by the camera bump and the
    case. <b>3. Rim runout</b> — optional: a reading, roll the car half a wheel turn, another reading. Finished steps get a
    ✓ and are remembered.</li>
  <li>Press the phone against the bar, screen facing out — camber flows to the adjustment screen in real time. The
    “steady” tag means the reading can be trusted.</li>
  <li><b>Caster</b> (front wheels only): wheels on turn plates, brake held with a prop, “Measure caster”. The phone leads
    the way: straight, then “turn the steering wheel left” or “right” about 20°, then the other way — it measures the steer
    angle with its gyroscope.</li>
  <li><b>Toe</b> — the tab explains why a phone on the rim cannot measure it, and one button turns the phone into a
    camera with targets.</li>
</ol>
<p class="muted">A phone held to the rim does not measure toe: toe is a rotation of the wheel about the vertical, and the
  accelerometer senses only tilt — gravity does not change under such a rotation. The gyroscope does sense it, but drifts —
  within a minute by more than the whole toe tolerance. So take toe by string or by camera: the same phone on a tripod
  works as a camera in live mode (see “Camera and targets”). Two phones on the wheels measure two wheels at once, four —
  all four. A phone connected by mistake is removed with “Disconnect” in the list.</p>
<p><b>Accuracy check</b> — the fourth, optional step of “Preparing the phone”. The phone goes on the table screen up and
  down again, then on the bar upright and upside down — but the calibration is compared, not changed. “Zero” shows how far
  the accelerometer's zero has moved since calibration (it does in the cold and with age), “mount” — how far the phone has
  shifted on the bar (a new case, another bar). Over 0.1° and the phone tells you which calibration step to repeat.
  Professional inclinometers are checked the same way: turned over by 180°, the zero offset changes sign and the angle
  does not.</p>
<p><b>The gyroscope of older phones.</b> Chrome on Android before version 66 gave the rotation rate in radians rather than
  degrees — the wheel's steer angle would come out 57 times too small. The program finds this out from the turn-over during
  calibration (always at least 180°) and converts. If caster comes out negative, the phone warns: most often the steering
  wheel was turned the wrong way first.</p>
<p><b>Set up once — then it just works.</b> The program remembers that access is on: after a restart it switches on again,
  the address and the code stay the same, and a phone with the page open reconnects by itself, keeping its wheel and its
  calibration. You can add the page to the phone's home screen. If someone else has seen the code, press “New key”: every
  phone is disconnected and each has to scan the new code. When the phones are no longer needed, “Switch Wi-Fi access
  off”.</p>`],

['optical', 'Measuring: camera and targets', `
<p>A printed chessboard on a rigid sheet can be fixed to the wheel as crookedly as you like — the program finds the wheel's
  axis of rotation from a series of photos, and the mounting error drops out.</p>
<ol>
  <li><b>Camera calibration</b> (once per camera and zoom): 10–20 photos of a chessboard at different angles and in all
    corners of the frame. Measure the square size on the printout with calipers — printers scale. Save
    <code>camera.json</code>.</li>
  <li><b>Camber from photos:</b> wheel lifted, target on the rim, camera strictly level at the side. 4–6 photos, turning
    the wheel 10–20° between them.</li>
  <li><b>Full measurement:</b> every frame must show a floor target as well as the wheel target — it defines the road plane
    and ties the four wheels into one coordinate system. It gives camber, individual toe and the thrust angle. There are
    two floor targets (front and rear) of different sizes; photos showing both link them.</li>
</ol>
<p><b>Live mode</b> (tab “4. Live mode”) — as on a professional 3D aligner: a phone on a tripod watches the wheel and the
  floor target, and camber and toe update on the adjustment screen 3–4 times a second while you turn the tie rod. On the
  phone — the “Phone as a camera” button:</p>
<ol>
  <li><b>Print the targets</b> — on the “4. Live mode” tab, the “PDF: wheel targets” and “PDF: floor targets” buttons. Print
    at 100 % scale and check the 100 mm ruler at the bottom of the sheet. Glue the targets to a rigid flat board (plywood,
    chipboard, composite). Each wheel has its own target of a different size — the program recognises which wheel it sees,
    there is no need to choose it on the phone. The floor targets are large: it is easier to order them as one sheet from a
    print shop.</li>
  <li><b>Camera calibration</b> — once per phone: show it the wheel target at different angles and in different parts of the
    frame until the bar fills. Hold the phone in landscape — and the same way later when measuring.</li>
  <li><b>Runout</b> — for each wheel: lift it and slowly turn it by hand a quarter turn or more. The program remembers how
    the wheel's axis sits relative to the target, and from then on a single frame is enough.</li>
  <li><b>Measure</b> — car on the floor. Show the camera all four wheels in turn (and two or three frames showing both floor
    targets), then toe appears. After that, put the phone by the wheel you are adjusting.</li>
  <li><b>Caster by camera</b> — front wheels on turn plates, the brake pedal held down with a prop so the wheel cannot
    roll. “Caster by camera” mode on the phone, slowly steer 15–20° one way, then the other. The program finds the wheel's
    steering axis and shows caster and steering axis inclination.</li>
</ol>
<p class="muted">Details, target sizes and photography tips are in OPTICAL.md in the repository.</p>`],

['sensor', 'Measuring: a sensor of your own', `
<p>Any device on your network — a home-made ESP32 head, a laser sensor, another program — can send angles to the program
  as a plain HTTP request with JSON. The format is on “Measurement → Own sensor” and in SENSORS.md. The program smooths
  the readings, detects when they have settled and shows the sensor in the status bar at the top.</p>`],

['live', 'Step 4. The adjustment screen', `
<ul>
  <li><b>Figures.</b> Green — in spec (<b>✓</b>), amber — in spec but near the limit (<b>!</b>), red — out of spec
    (<b>✕</b>), white — no specification, grey — no reading. Above the figure is the tolerance scale: the green zone and a pointer at the current value.</li>
  <li><b>Views.</b> F2 cycles between the overview and large front- and rear-axle views; F3 and F4 jump straight to an
    axle. Under the car the large view is easier: the figures are readable from two or three metres.</li>
  <li><b>Units</b> — degrees and minutes or decimal degrees; toe can be shown in millimetres (at the rim diameter from the
    Vehicle step).</li>
  <li><b>“Readings steady”.</b> While values are changing the screen says “readings changing…”. Record figures and
    tighten lock nuts only when steady.</li>
  <li><b>Tap any angle</b> — a panel opens on the right: what the angle is, what adjusts it on your suspension, what the
    data says for this model.</li>
  <li><b>The “how far to turn” helper.</b> In the same panel: press “Remember position”, turn the tie rod or eccentric by
    a known amount (¼, ½, 1 turn) and press how far you turned it. The program works out what one turn does on your car
    and from then on tells you: “to nominal ≈ 1¼ turns, the same way”.</li>
  <li><b>“What next”</b> — the line under the verdict. It shows the next step: the “before” snapshot, then the angle to
    adjust now, in the right order (rear axle → caster → camber → toe), and how far it is from the spec; then the “after”
    snapshot and the report. The button on the right does it or opens the angle's panel. If the angle has no adjuster on
    your suspension, the line says so — look for wear or a bent part.</li>
  <li><b>The source bar</b> at the top shows where data comes from: phone, sensor, camera, manual entry, demo.</li>
  <li><b>Sound and voice</b> — the buttons next to the units, keys <kbd>F7</kbd> and <kbd>F8</kbd>: guidance when you
    cannot see the screen (see “Settings and accessibility”).</li>
</ul>`],

['order', 'Adjustment order — do not change it', `
<ol>
  <li><b>Rear axle</b> (camber, then toe) — it sets the thrust line from which front toe is measured.</li>
  <li><b>Caster</b> — changing it moves both camber and toe.</li>
  <li><b>Camber</b> — it moves toe.</li>
  <li><b>Toe — last.</b> Steering wheel dead straight and locked. Set the toe of <i>each</i> wheel, not just the total: the
    total can be reached in endless ways, and only one leaves the steering wheel straight. Turn both tie rods by the
    same amount in opposite directions.</li>
  <li><b>Check measurement.</b> Tighten the lock nuts (tightening often shifts toe), roll the car 3–5 m, bounce the body
    and measure again. An alignment without a check measurement is not an alignment, it is a hope.</li>
</ol>
<p><b>Which way to turn the tie rod if you don't know:</b> a quarter turn, then watch the pointer. The live screen answers
  that question faster than any diagram.</p>
<p>Factory procedures use the same order: Hyundai bulletin 20-SS-001H says to set rear camber and toe first, then centre and
  lock the steering wheel, and only then adjust front caster, camber and toe; the Tesla manual likewise goes rear camber,
  rear toe, front camber and caster, front toe. Set angles to the centre of the specification, not just “inside it”.</p>
<p><b>Steering wheel crooked although toe is in spec?</b> Then the toe is split unevenly between the wheels. The front toe
  panel on the adjustment screen tells which way the steering wheel will point — set the steering wheel straight and bring
  each wheel to its own nominal.</p>
<div data-figs="thrust tieRod"></div>`],

['report', 'Step 5. The report', `
<p>The report is built from two snapshots: “before” — ahead of adjustment, and “after” — once everything is set. F5 on the
  adjustment screen takes a snapshot in one press (“before” first, then “after”). A snapshot needs camber and toe of
  all four wheels.</p>
<p>You can add the plate number or VIN, the mileage and your name. “Print / PDF” opens the system print dialog — choose
  “Save as PDF” to get a file. The report lists the source of the specifications, the adjustment order and remarks
  (rim runout, included-angle differences and so on).</p>
<p>Finished with the car — <b>“New vehicle”</b>: readings, snapshots, the checklist and the report fields are cleared, and
  the next car starts with a clean sheet.</p>`],

['tyres', 'What tyre wear tells you', `
<p>Before measuring, look at the front and rear tyres and run your palm over the tread: wear often tells at once which
  angle is off or what is worn. Write down what you see — after the alignment you will have something to compare with.</p>
<div data-wear></div>
<p class="muted">From MOOG's “Understanding Tire Wear” and Hyundai bulletin 20-SS-001H. Feathering is easier to feel than to
  see: run your palm across the tread one way and the other — on one side the block edges are sharp. Cupping is not an
  alignment matter: dampers, wheel balance and play come first.</p>`],

['diag', 'Diagnosis: what is bent', `
<p>Camber and toe are adjusted, but steering axis inclination (SAI) and the included angle (SAI + camber) are set by the
  parts. If they are out of spec or differ left to right, adjustment will not help — something is bent. Compare the two
  sides with each other.</p>
<table class="params"><thead><tr><th>Camber</th><th>SAI</th><th>Included angle</th><th>Probable cause</th></tr></thead><tbody>
  <tr><td>out of spec</td><td>in spec</td><td>in spec</td><td>The knuckle is sound. A misadjusted cam bolt, a shifted
    subframe, a bent control arm or a displaced strut tower.</td></tr>
  <tr><td>out of spec</td><td>out of spec</td><td>in spec</td><td>The knuckle is sound. A bent strut, a bent control arm or a
    displaced upper strut mount.</td></tr>
  <tr><td>out of spec</td><td>any</td><td>out of spec</td><td>A bent steering knuckle (spindle) — the knuckle is replaced.</td></tr>
</tbody></table>
<p>Per Hyundai bulletin 20-SS-001H: an SAI difference between left and right of more than 1° means damaged parts; lower SAI
  on one side often means a bent lower control arm, higher — a damaged upper strut mount. SAI from a wheel sweep is not
  precise: a 0.1° zero error of the gauge gives almost 2° of SAI, so confirm such a finding with the camera.</p>
<p><b>Steering lock angles.</b> On turn plates, steer until the outer wheel stands at 20° and note the angle of the inner
  one; then the other way (“Measurement → Steering lock angles”). The inner wheel usually turns further than the outer,
  but what matters is that left and right match: a difference over 1° means a bent steering arm or tie rod, even when toe is
  in spec. Such a car is repaired, not adjusted.</p>
<p class="muted">The table follows study material for the ASE A4 exam; the exact SAI, included angle and lock angles for your
  model are in its workshop manual.</p>`],

['road', 'Road test', `
<p>An alignment is checked on the road. The procedure from Hyundai bulletin 20-SS-001H:</p>
<ol>
  <li>A straight, level stretch at least 250 m long, no ruts, wind or noticeable crown; a lane 3–3.5 m wide.</li>
  <li>Tyre pressures to specification, only the driver in the car.</li>
  <li>About 64 km/h (40 mph). Set the steering wheel so the car goes straight and let it go lightly.</li>
  <li>Time how many seconds the car takes to drift one metre — from one side of the lane to the other. <b>More than 6 s
    passes, 6 s or less fails.</b></li>
  <li>Repeat in the opposite direction on the same road: if the drift changes side, the road or the wind is to blame, not
    the car.</li>
  <li>If the car goes straight but the steering wheel is crooked, redo the toe with the steering wheel locked straight.</li>
</ol>
<div data-figs="roadTest"></div>
<p>The result goes into the report — the “Road test” section at the bottom.</p>`],

['susp', 'Suspension types', `
<p class="muted">Every model has its own factory specification, but there are only about a dozen suspension designs in the
  world. The design decides what can be adjusted at all, with what, and what to check before adjusting.</p>
<div id="gSusp"><span class="spin"></span></div>`],

['trucks', 'Trucks, buses, vans', `
<ul>
  <li>Kingpin front beam axle: <b>camber and kingpin inclination are not adjustable</b> — a deviation means a bent beam,
    worn kingpins or bearings.</li>
  <li><b>Caster</b> is corrected with taper shims between the leaf spring and the beam pad — the same on both sides.</li>
  <li><b>Toe</b> is set by the cross tie rod — total toe only. The steering wheel is centred with the length of the drag
    link (pitman arm to steering arm).</li>
  <li>Twin rear wheels count as one wheel — measure the outer one; both tyres at the same pressure.</li>
  <li>Truck manuals often give toe in mm at the tyre or at the rim — enter the diameter the figure refers to.</li>
  <li>Multi-axle vehicles (rear bogie) are treated as two-axle for now: the front axle and one rear axle. Check the
    bogie axles for parallelism separately.</li>
</ul>`],

['bent', 'When adjustment will not help', `
<ul>
  <li><b>Different included angle</b> (steering axis inclination + camber) left to right by more than 1° — a bent strut,
    knuckle or arm. Camber and SAI may each “lie”, their sum does not.</li>
  <li><b>Different wheelbase</b> left to right (more than 10 mm) — a deformed body, frame or arms.</li>
  <li><b>Large setback</b> of the wheels along the car (more than 12 mm) — a deformed subframe or chassis rail.</li>
  <li><b>Thrust angle</b> on a rigid axle or beam — there is no adjustment: a sheared spring centre bolt, loose U-bolts,
    a bent axle.</li>
  <li><b>Camber wanders</b> from one reading to the next — play in kingpins, ball joints or bushings. Repair first.</li>
</ul>`],

['data', 'Where the specifications come from', `
<p>Complete factory specification databases are paid commercial products, and figures must never be made up: a wrong
  camber means tyres eaten in one season and a car that behaves badly in an emergency. So every entry in the program
  names its source:</p>
<ul>
  <li><span class="badge factory">Factory manual</span> — checked against the document, edition and page given.</li>
  <li><span class="badge community">Community</span> — cross-checked against an independent source.</li>
  <li><span class="badge unverified">Unverified</span> — from a single source, shown with a warning.</li>
  <li><span class="badge catalog">Design only</span> — the suspension is known, the tolerances are not; compared with the class guidance.</li>
  <li><span class="badge class_guidance">Class guidance</span> — typical values for the design, <b>not</b> for your model.</li>
</ul>
<p>Have a workshop manual? The “Enter specs” button on the left: the program checks the figures for typos, keeps them on
  your computer and prepares a file you can send to the project so that every owner of that model can use it. Give the
  edition and page: without a source an entry is not accepted into the shared database.</p>`],

['trouble', 'If something does not work', `
<table class="params"><thead><tr><th>What happens</th><th>What to do</th></tr></thead><tbody>
  <tr><td>No window, the program opened in the browser</td><td>Microsoft Edge WebView2 Runtime is missing (happens on older
    Windows 10). Install it from Microsoft's site — or keep using the browser, it is the same program.</td></tr>
  <tr><td>The phone cannot open the address</td><td>Are phone and computer on the same network? Guest Wi-Fi often isolates
    devices — connect both to the phone's hotspot. In Windows the network must be “Private” and the firewall must allow
    the program on private networks.</td></tr>
  <tr><td>“Connection is not private” on the phone</td><td>That is expected: the certificate was made by the program.
    “Advanced” → “Proceed to the site”.</td></tr>
  <tr><td>“Link expired”</td><td>The key in the address was changed with “New key”. Scan the new QR code.</td></tr>
  <tr><td>The phone says “no connection to the computer”</td><td>The program on the computer is closed or the phone has left the
    Wi-Fi network. As soon as the program is running again and the phone is on the same network, the connection comes back
    by itself — no need to scan again. If the computer's network address has changed, scan the code once more.</td></tr>
  <tr><td>The phone's sensors do not respond</td><td>Open the page in Chrome (Android) or Safari (iPhone); on an iPhone allow
    motion sensor access. Browsers built into messengers do not provide the sensors.</td></tr>
  <tr><td>Figures jump around</td><td>The phone is not pressed to the bar, the bar rests on the tyre, wind shakes the string,
    someone is sitting in the car. Wait for “readings steady”.</td></tr>
  <tr><td>Camera calibration is “poor”</td><td>More photos, the board in every corner of the frame and tilted, the board
    glued to a rigid flat sheet, the square size measured with calipers.</td></tr>
  <tr><td>Caster came out negative</td><td>Most often the steering wheel was first turned the wrong way. For the left wheel —
    left first, then right; for the right wheel — right first. Measure caster again.</td></tr>
  <tr><td>The phone's accuracy check failed</td><td>“Zero” — redo the phone calibration on the table (step 1); “mount” — the
    mount calibration on the bar (step 2). Check that the bar is straight and pressed against the rim flanges.</td></tr>
  <tr><td>You found a bug</td><td>Describe it in Issues on GitHub and attach <code>wheelalign.log</code> from the data folder.</td></tr>
</tbody></table>`],

['sources', 'Where the method comes from', `
<p>The working order, the checks and the thresholds in the program come from manufacturers' open documents and training
  material, not invention:</p>
<ul>
  <li><b>Hyundai, bulletin 20-SS-001H</b> “Wheel Alignment Requirements and Best Practices” (2020, published by NHTSA):
    the adjustment order, pull from camber and caster, the 0.5° camber difference, the 1° SAI difference, the road test.</li>
  <li><b>Tesla, Model 3 service manual</b>, “Four Wheel Alignment Check and Adjust”: preparation, load, ride height, brake
    pedal depressor, steering wheel lock, rear-then-front order, road test.</li>
  <li><b>MOOG (Federal-Mogul), “Understanding Tire Wear”</b>: tyre wear and its causes.</li>
  <li><b>Study material for the ASE A4 exam</b> (suspension and steering): pre-alignment inspection, play checks, the dry
    park test, SAI and included angle diagnosis, steering lock angles.</li>
  <li><b>Analog Devices, AN-1057</b> “Using an Accelerometer for Inclination Sensing”: calibrating a tilt sensor by turning
    it over by 180° — the phone's calibration and check rest on it.</li>
  <li><b>W3C, “Device Orientation and Motion”</b>, and the Chromium change for Chrome 66: rotation rate in degrees per
    second; before Chrome 66 on Android — in radians.</li>
</ul>
<p class="muted">Your car's factory tolerances come only from its workshop manual: see “Where the specifications come
  from”.</p>`],

['safety', 'Safety', `
<ul>
  <li>Get under a car only on sound axle stands. A jack is a lifting device, not a support.</li>
  <li>Brakes, steering and suspension are what lives depend on. Not sure about the assembly — find someone who is.</li>
  <li>After adjustment: the first drive at low speed on an empty road. Is the steering wheel straight, does the car pull?</li>
  <li>Tie-rod lock nuts tight, split pins in place.</li>
</ul>`],

['keys', 'Keys', `
<table class="params"><tbody>
  <tr><td><kbd>F1</kbd></td><td>Guide</td></tr>
  <tr><td><kbd>F2</kbd>…<kbd>F6</kbd></td><td>Vehicle, preparation, measurement, adjustment, report (the adjustment screen has its own)</td></tr>
  <tr><td><kbd>F12</kbd></td><td>Next step</td></tr>
  <tr><td colspan="2" class="muted">On the adjustment screen:</td></tr>
  <tr><td><kbd>F2</kbd></td><td>Change view</td></tr>
  <tr><td><kbd>F3</kbd> / <kbd>F4</kbd></td><td>Front / rear axle, large</td></tr>
  <tr><td><kbd>F5</kbd></td><td>“Before” snapshot, then “after”</td></tr>
  <tr><td><kbd>F6</kbd></td><td>Report</td></tr>
  <tr><td><kbd>F7</kbd> / <kbd>F8</kbd></td><td>Sound signals / voice for the chosen angle</td></tr>
  <tr><td><kbd>F9</kbd> / <kbd>F10</kbd></td><td>Demonstration / auto-adjust: the program sets the angles itself, step by step</td></tr>
  <tr><td><kbd>←</kbd> <kbd>→</kbd></td><td>In the demonstration — “turn” the selected angle (with Shift — in bigger steps)</td></tr>
  <tr><td><kbd>Esc</kbd></td><td>Close the side panel</td></tr>
  <tr><td colspan="2" class="muted">Everywhere:</td></tr>
  <tr><td><kbd>Tab</kbd> / <kbd>Shift</kbd>+<kbd>Tab</kbd></td><td>To the next / previous element</td></tr>
  <tr><td><kbd>Enter</kbd>, Space</td><td>Press a button, open the chosen angle</td></tr>
</tbody></table>`],

  ],
};
