---
id: TASK-9163-001
title: "Add local test and coverage instructions to README"
task_type: documentation
parent_review: TASK-REV-9163
feature_id: FEAT-9163
wave: 1
implementation_mode: direct
complexity: 2
dependencies: []
---

Add a short section to the project README that explains how to run the web app's tests and generate its coverage report locally, naming only the commands the project already declares, and leaving every other part of the README unchanged.

## The words of the request this task serves

List lines: 1, 2, 3, 4

> Add a short section to the README that explains how to run the web app's tests and its coverage report locally, using the commands the project already declares; change nothing else.

This task is the whole of the request: it adds the new section (lines 1–2), uses only declared commands (line 3), and confines the change to the new section (line 4).

## Files to Create

- _none_

## Files to Modify

- `README.md`

## Acceptance Criteria

- [ ] The README contains a new short section explaining how to run the web app's tests locally.
- [ ] That section names the declared test command exactly as the project declares it (copied verbatim from the project configuration — e.g. the `test` script in `apps/web/package.json` or the equivalent declared command in `scripts/check.sh`).
- [ ] The README contains a new short section (or the same section) explaining how to generate the web app's coverage report locally.
- [ ] That section names the declared coverage command exactly as the project declares it (copied verbatim from the project configuration — e.g. the `coverage` script in `apps/web/package.json` or `scripts/coverage.sh`).
- [ ] Every command shown in the new section is a command the project already declares; the section introduces no new command, tool, or workflow.
- [ ] The rest of the README is unchanged from its previous content — only the new section is added.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- Open `README.md` and locate the appropriate insertion point (typically near the end, after any existing "Getting Started" or "Development" section, before any license or contribution footer).
- Read `apps/web/package.json` to find the declared `test` and `coverage` (or similarly named) scripts. Copy the command strings verbatim — do not paraphrase, reformat, or substitute.
- If the web app's test or coverage commands are declared in `scripts/check.sh` or `scripts/coverage.sh` rather than (or in addition to) `apps/web/package.json`, use those declared forms. The project's own configuration is authoritative.
- Keep the section short: a heading, one or two sentences of context, and the two commands in fenced code blocks.
- Do not modify any line outside the new section. Use a targeted edit (insert the new block) rather than rewriting the file.
- The four low-confidence assumptions (ASSUM-001 through ASSUM-004) are deferred; the coder must verify the declared commands by reading the actual configuration files before writing the section. If a declared command is not found, stop and flag rather than inventing one.