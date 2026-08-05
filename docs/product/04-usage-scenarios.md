# Where & How It's Used

## Where

- **Local machine, primarily.** Recording happens on the developer's own
  desktop (or inside their dev environment) — browser, terminal, IDE,
  native apps.
- **Consumption happens elsewhere**: inside a PR description on GitHub/
  GitLab, a ticket in Jira/Linear, a Slack message, or a repo's `/docs`
  folder as a checked-in Markdown file.
- Not (initially) a hosted web app people log into to browse a library —
  that's a Phase 3 question, not an assumption baked into the core design.

## How — 3 real-world walkthroughs

### Scenario A: Solo developer documents a new feature for a PR
1. Dev finishes building a new "export to CSV" button.
2. Runs `loomless record start "export-csv-feature"`.
3. Clicks through the feature in the browser: opens the page, clicks
   Export, picks a date range, downloads the file, opens the CSV to show
   it's correct.
4. Runs `loomless record stop`.
5. Runs `loomless export --format md`, gets `export-csv-feature.md` with 5
   steps, each with a screenshot and an auto-generated caption.
6. Pastes the Markdown into the PR description. Reviewer sees exactly what
   the feature does without pulling the branch.

**What matters here:** speed (under a minute of overhead), and that the
output is plain Markdown a reviewer can read inline on GitHub without
opening another tool.

### Scenario B: AI agent documents a debugging session for review
1. Developer asks Claude Code to fix a failing test.
2. Claude Code investigates: reads the stack trace, greps the codebase,
   finds the bug, runs the test again to confirm the fix.
3. Via MCP, the agent calls `start_recording`, then `capture_step(note)`
   after each meaningful action — "ran the failing test to see the exact
   error," "found the off-by-one in `paginate()`," "re-ran the test suite,
   all green" — with the terminal output/screenshot captured alongside
   each note.
4. Agent calls `generate_story(format=markdown)` and includes the result
   in its final response, or attaches it to the PR it opens.

**What matters here:** the story reads like a debugging narrative a human
engineer would write, not a flat list of clicks — because the agent
supplied the actual reasoning, not inferred captions.

### Scenario C: Bug repro handoff between engineers
1. QA engineer finds a bug: a form silently fails to save on a specific
   combination of inputs.
2. Records a short session reproducing it: fills the form, clicks Save,
   shows the silent failure (no error, no save), opens dev tools to show
   the failed network request.
3. Exports to Markdown, attaches to the bug ticket.
4. Assigned engineer opens the ticket, sees the exact repro steps with
   screenshots (including the failed network call), doesn't need to ask
   "can you reproduce this on a call with me?"

**What matters here:** precision and trustworthiness — the engineer reading
this needs to believe the steps are exactly what happened, not paraphrased
or reordered after the fact (ties into `06-editing-and-versioning.md`).
