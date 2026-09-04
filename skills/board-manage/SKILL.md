---
name: board-manage
description: Kanban board management protocol — when and how to create, move, and manage cards on the Cyber Mango board.
---

# Board Management Protocol

You have access to a Cyber Mango kanban board via MCP tools. This skill defines exactly how and when you use them. Follow these rules without exception.

## Session Start

The SessionStart hook already injects the board summary; do not re-fetch it unless you need fresh data after writes.

## Project Tagging

Every card must be tagged with the current project name. Before creating a card, detect the project name by running:

```bash
git remote get-url origin 2>/dev/null
```

Extract the repository name from the URL (the last path segment, without `.git`). For example:
- `https://github.com/juandagalo/cyber-mango.git` -> `cyber-mango`
- `git@github.com:juandagalo/my-app.git` -> `my-app`

Pass the extracted name as the `tags` parameter when calling `create_card`. If additional tags are needed (e.g., `bug`, `feature`), combine them: `tags: "my-project,bug"`.

If there is no git remote (not a git repo), omit the project tag.

## When to Create Cards

Create a card whenever:
- The user mentions starting a new feature, bug fix, task, refactor, spike, or investigation
- The user mentions work they are about to do or have been doing
- A concrete action item emerges from the conversation that someone is responsible for

Do not wait for the user to explicitly ask you to create a card. If the user says "I'm going to fix the login bug", create the card proactively, then confirm it was created.

Before creating, search engram for the topic and check the board so you never create a duplicate card. If a matching card exists, update it instead.

## Card Format

Title and description formats are defined in the `create_card` and `update_card` parameter descriptions; follow them exactly. Every card uses the What/Why/Context description; ticket details go inside `## Context`.

The current state of a card (status, progress, blockers) is tracked via columns, phases, and tags, not in the description. A card description must stand alone without the surrounding chat history.

## Terminology Mapping

Users may refer to cards as "tickets", "tasks", "items", or "work items". These all map to **cards** on the board. When the user says "move the ticket to Done" or "update the task", they mean a card operation.

## Placement

Call `get_board_summary`; each column's `description` says what state it represents. Pick the column whose description fits the current work state. Do not assume column names map to a fixed meaning. If a column has no description, infer its purpose from position (first column is intake, last column is terminal) and then from its name. When you have inferred it with confidence, persist it with `update_column` so later sessions do not have to guess.

Work moves left to right by position. Never skip columns without a stated reason; if a card jumps from the first column to the last, confirm with the user.

When metadata also changes (title, description, priority, phase), pass `column_name` to `update_card` so the move happens in the same call. Use `move_card` only for a pure move or reposition. Do not assume the card is already in the right column; verify first.

When work is blocked or paused, move the card back to the column describing ready-to-start work and add the `blocked` tag.

## Priority Convention

Assign priorities based on urgency and impact:

- **low**: Nice-to-have, exploratory, no deadline pressure. Default for spikes and research.
- **medium**: Normal work items with no special urgency. This is the DEFAULT priority when none is specified.
- **high**: Blocking other work, has a hard deadline, or is important enough that delay has real consequences.
- **critical**: Production incidents, security vulnerabilities, data loss risks, or anything that requires immediate action regardless of other work.

If the user uses words like "urgent", "blocking", or "ASAP", use **high**. If they mention production, outages, or security breaches, use **critical**.

## Tag Conventions

Use tags to classify cards with additional context:

- `bug`: Something is broken and needs fixing
- `feature`: New functionality being added
- `chore`: Maintenance, tooling, dependency updates, refactors with no behavior change
- `blocked`: Work cannot proceed until something else resolves
- `spike`: Time-boxed investigation or proof of concept with no guaranteed deliverable

Assign tags via `manage_tags`. A card can have multiple tags; use them consistently. Tag writes are logged to the activity log and appear in the session summary, so tag deliberately.

## WIP Limit Enforcement

Before adding a card to a column that has a WIP limit, read `card_count` and `wip_limit` from `get_board_summary`. If the column is at capacity:

1. Warn the user explicitly: "The [column name] column is at its WIP limit of [N]. Adding another card would exceed it."
2. Ask if they want to proceed anyway or move an existing card first.
3. Do not move the card until the user confirms.

Never silently exceed a WIP limit.

## Phases

Every board has workflow phases that track where a card is in the delivery pipeline. The default phases are: Development, Code Review, QA, Client Review, Ready to Deploy.

Assign a phase when creating or updating a card if the work state is clear:

- The user says "I'm coding this" or "working on implementation" -> **Development**
- The user opens a PR or asks for a review -> **Code Review**
- The user says "ready for testing" or "needs QA" -> **QA**
- The user says "waiting on the client" or "sent for approval" -> **Client Review**
- The user says "approved", "ready to ship", or "merge it" -> **Ready to Deploy**

If the work state is ambiguous, do not assign a phase. A card without a phase is valid; it means the delivery stage is unknown.

Phases and columns serve DIFFERENT purposes. **Columns** track the workflow state of the TASK; **phases** track the delivery stage of the WORK. A card can stay in the same column while its phase changes several times, so phases change more often than columns.

When you detect a phase change, update the card immediately with `phase_name` (add `column_name` if the column should change too; use `unset_phase` to remove a phase). Do not skip phase transitions without reason; if a card jumps from Development to Ready to Deploy, confirm with the user.

## Errors

`NOT_FOUND:` means the id does not exist. Any other error is a real database failure: report it, do not retry with different ids.
