# ExamServer

[![build](https://github.com/cmpxchg16b-nop/dcna-questions/actions/workflows/build.yml/badge.svg)](https://github.com/cmpxchg16b-nop/dcna-questions/actions/workflows/build.yml)
[![coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/cmpxchg16b-nop/dcna-questions/main/.github/badges/coverage.json)](https://github.com/cmpxchg16b-nop/dcna-questions/actions/workflows/build.yml)

An exam practice site, built with simplicity and brevity in mind.

It is a small, general-purpose practice site for many kinds of
exam you can describe in a plain XML file.

## What it does

- **Practice exams in the browser.** Pick an exam, answer its questions in a
  timed session, submit, and immediately see your score and which answers were
  right or wrong.
- **Question banks are plain XML files.** An exam is a single document with a
  title, a passing score, and a set of questions — single- or multiple-choice,
  with optional image exhibits. No database or authoring tool required; see
  `exams/` for examples and `exams/exam.xsd` for the schema.
- **Randomized retakes.** An exam can draw from multiple question collections,
  so each attempt can present a different selection of questions.
- **Track your history.** Finished sessions produce reports you can review
  later, so you can watch your scores improve over repeated attempts.
- **Bring your own exams.** Signed-in users can upload their own exams as
  _exam-packs_ — portable tarballs — and practice them alongside the built-in
  ones.
- **Painless sign-in.** Practice anonymously as a visitor with one click, or
  sign in with GitHub or any OIDC provider to keep your uploads and history
  tied to an identity.
- **Dark mode.** Light and dark themes that follow your system setting, or
  flip between them yourself with the toggle in the top bar.
- **Responsive layout.** The interface adapts to your screen, so practicing on
  a phone works as well as on a desktop.

## Exam-pack (portable exam document)

An _exam-pack_ is a single tarball that bundles everything one exam needs:
exactly one exam XML document (always named `exam.xml`), plus the images its
exhibits reference. One pack, one exam — it is the unit by which exams move
around. In other words, an exam-pack is also a portable, and, self-contained
exam document.

Upload an exam-pack from the site's uploads section, tick **Associate**, and
its exams appear in your exam list immediately — no server restart, no manual
file placement. De-associate to hide them again, or delete the upload
entirely. Uploaded exams are private to the account that uploaded them.

Because an exam-pack is self-contained, sharing exams between users is just
sharing a file: hand the tarball to someone else and they can upload it to
their own account (or their own server) and start practicing. The `exam-packs/`
directory ships a few ready-made packs you can try this with.

## The exam XML format

Each exam is one XML document, validated by `exams/exam.xsd`. The skeleton:

```xml
<exam id="bcnsce-bc-01" shortname="BCNSCE" code="BC-01">
  <title>Basic Computer Network Skills Certification Exam</title>
  <description>A 120-minute exam that tests ...</description>
  <passingscore>4.0</passingscore>
  <examcategory>certification-exam</examcategory>
  <questionset>
    <questioncollection>
      <!-- questions go here -->
    </questioncollection>
  </questionset>
</exam>
```

The metadata: `id`, `shortname`, and `code` identify the exam; `title` and
`description` are shown in the exam list; `passingscore` is the score needed
to pass; `examcategory` is `certification-exam` or `practice-exam` (described
below).

### Certification exams vs. practice exams

The category is not just a label — it changes how a session behaves.

A **certification exam** simulates a proctored, high-stakes sitting. Questions
are served one at a time in a fixed sequence: no jumping back and forth, no
session options to tweak, and no answer key — the graded report shows your
score and pass/fail, but never reveals the questions or their correct
answers.

A **practice exam** is built for learning. You can customize the session
before starting (including free navigation between questions), press **Check**
on a question to see immediately whether your answer is right, revisit
questions to find your earlier answers still in place, and the finished
report embeds each question you answered together with its correct answer, so
you can review your mistakes.

The `questionset` holds one or more `questioncollection` elements. With
several collections, each session draws a random one — that is how randomized
retakes are expressed.

Every question carries an `id`, a `type`, a `score`, a `<description>`,
optional `<exhibits>` (described below), and a `<correctanswer>` whose shape
depends on the type.

### Single-choice

Options plus the correct one. (Listing more than one correct option means any
of them is accepted.)

```xml
<question id="1" type="single-choice" score="1">
  <description>Which protocol dynamically assigns IP addresses? (Choose one.)</description>
  <options>
    <option id="1">DNS</option>
    <option id="2">DHCP</option>
    <option id="3">NAT</option>
  </options>
  <correctanswer>
    <options>
      <option id="2" />
    </options>
  </correctanswer>
</question>
```

### Multiple-choice

Same shape; the answer lists the full set of options that must be selected.

```xml
<question id="2" type="multiple-choice" score="1">
  <description>Which two of the following are names of operating systems? (Choose two.)</description>
  <options>
    <option id="1">Cisco</option>
    <option id="2">Linux</option>
    <option id="3">Google</option>
    <option id="4">macOS</option>
    <option id="5">Microsoft</option>
  </options>
  <correctanswer>
    <options>
      <option id="2" />
      <option id="4" />
    </options>
  </correctanswer>
</question>
```

### Drag-and-drop

Draggable `<candidates>` on the left, `<drops>` targets on the right; the
answer is a set of connections, and `requiredUniqueConnections` says how many
of them the taker must make.

```xml
<question id="3" type="drag-and-drop" score="1">
  <description>Drag each protocol onto the transport category it belongs to. Not all protocols are used.</description>
  <candidates>
    <candidate id="1">TCP</candidate>
    <candidate id="2">UDP</candidate>
    <candidate id="3">ICMP</candidate>
  </candidates>
  <drops>
    <drop id="1">Stream-oriented protocol</drop>
    <drop id="2">Datagram protocol</drop>
  </drops>
  <correctanswer>
    <connectionsolutions>
      <connectionsolution requiredUniqueConnections="2">
        <connect src="1" dst="1" />
        <connect src="2" dst="2" />
      </connectionsolution>
    </connectionsolutions>
  </correctanswer>
</question>
```

Two variants exist: `<multiareadrop>` groups the drop targets under labeled
areas, and `<imgDragAndDrop>` turns the candidates into image snippets dragged
onto positions over a background image. Multiple `<connectionsolution>`
elements express alternative correct answers, and `<connectcombination>` is
shorthand for "any pairing of these sources and destinations".

### Exhibits

A question description can refer to a diagram — the classic "Refer to the
exhibit." phrasing. The `<exhibits>` element carries those diagrams: one or
more `<exhibit>` entries, each wrapping an `<image>` whose `src` points at a
picture bundled with the exam (for an exam-pack, a file in its `assets/`
directory).

```xml
<question id="4" type="single-choice" score="1">
  <description>Refer to the exhibit. Why do the pings to fdba:6c76:8f12:: fail?</description>
  <exhibits>
    <exhibit>
      <image src="assets/bgp-topology.png" />
    </exhibit>
  </exhibits>
  <options>
    <!-- ... -->
  </options>
  <correctanswer>
    <!-- ... -->
  </correctanswer>
</question>
```

Exhibits work with any question type, and a question may carry several of
them.

There are more examples live in `exams/` .

## What it is (and isn't)

The whole site — frontend and backend — ships as **one self-contained binary**
with no database and no external services to run. State is kept in memory, so
restarting the server resets sessions and history. That tradeoff is deliberate:
the goal is a practice site you can run anywhere in seconds, not a production
learning-management system.

## Running it

The fastest way is the container image, which bundles the built-in exams:

```sh
docker build -t exam-server .
docker run --rm -p 8080:8080 -e JWT_SECRET=change-me exam-server
```

Then open <http://localhost:8080>, sign in as a visitor, and start a session.

To run from source instead:

```sh
cd web/exam-lab && npm ci && npm run build && cd ../..
JWT_SECRET=change-me go run ./cmd/server --assets-dir=assets --load-exam-dir=exams
```

Point `--load-exam` or `--load-exam-dir` at your own XML files to serve
different exams.

## Acknowledgements

Many appreciations for the DN42 domain exam.edu.dn42 and testcenter.edu.dn42 sponsored by nedifinita AS4242420454.

## Development

### Automatic End-to-End Tests Subset

The `e2e/` directory holds the **automated** end-to-end tests — tests that exercise the full HTTP stack against an in-process `httptest.Server` via `go test`.

It is not the only place e2e testing happens: manual e2e checks (e.g. running the server and driving it with `curl` or a browser) are performed outside this directory and are not covered here.

You are still encouraged to cook / orchestrate your own e2e testing flows.

#### Running the e2e suite

```sh
go test ./e2e/... -v -timeout 30s
```
