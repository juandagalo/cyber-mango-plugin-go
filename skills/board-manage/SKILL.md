---
name: board-manage
description: Kanban board management protocol — when and how to create, move, and manage cards on the Cyber Mango board.
---

# Board Management Protocol

You have access to a Cyber Mango kanban board via MCP tools. This skill defines exactly how and when you use them. Follow these rules without exception.

## Session Start Protocol

At the start of every session, call `get_board_summary` immediately. This gives you a snapshot of the current board state — how many cards are in each column, their priorities, and any WIP limits in effect. Do not wait for the user to ask. This context is required before you can answer any work-related question accurately.

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

Before creating any card, call `get_board` and search the results for an existing card that matches the work item. If one exists, update it instead of creating a duplicate.

## Column Definitions and Workflow

The default board has five columns. Use them as follows:

- **Backlog**: Ideas, future work, parked items, anything not yet committed to. Use this as the default column when the user mentions something without implying they are starting it now.
- **To Do**: Committed work that is ready to start. The user has decided this will be done in the near term.
- **In Progress**: Actively being worked on right now. Only one or a small number of cards should be here at once.
- **Review**: Work that is complete from the implementer's side but waiting for feedback, code review, QA, or approval.
- **Done**: Completed and verified. The acceptance criteria have been met.

Never skip columns without a stated reason. If a card jumps from Backlog to Done, that is a data quality problem unless the user explicitly confirms it is correct.

## Terminology Mapping

Users may refer to cards as "tickets", "tasks", "items", or "work items". These all map to **cards** on the board. When the user says "move the ticket to Done" or "update the task", they mean a card operation.

## Movement Protocol

Move cards when the work state changes:

- When you or the user start working on something: move to **In Progress**
- When implementation is complete and feedback is needed: move to **Review**
- When the work is accepted and verified: move to **Done**
- When work is blocked or paused: move back to **To Do** and add the `blocked` tag

When you detect a state transition, ALWAYS include `column_name` in your `update_card` call alongside any other changes. This ensures the card moves to the correct column in the same operation. Use `move_card` only when repositioning within a column or moving without other changes. Do not assume the card is already in the right column — verify first.

## Priority Convention

Assign priorities based on urgency and impact:

- **low**: Nice-to-have, exploratory, no deadline pressure. Default for spikes and research.
- **medium**: Normal work items with no special urgency. This is the DEFAULT priority when none is specified.
- **high**: Blocking other work, has a hard deadline, or is important enough that delay has real consequences.
- **critical**: Production incidents, security vulnerabilities, data loss risks, or anything that requires immediate action regardless of other work.

If the user does not specify a priority, use **medium**. If the user uses words like "urgent", "blocking", or "ASAP", use **high**. If they mention production, outages, or security breaches, use **critical**.

## Tag Conventions

Use tags to classify cards with additional context:

- `bug`: Something is broken and needs fixing
- `feature`: New functionality being added
- `chore`: Maintenance, tooling, dependency updates, refactors with no behavior change
- `blocked`: Work cannot proceed until something else resolves
- `spike`: Time-boxed investigation or proof of concept with no guaranteed deliverable

Assign tags via `manage_tags`. A card can have multiple tags. Tags help filter and prioritize the board — use them consistently.

## WIP Limit Enforcement

Before adding a card to a column that has a WIP limit, call `get_board` and count the current cards in that column. If the column is at capacity:

1. Warn the user explicitly: "The [column name] column is at its WIP limit of [N]. Adding another card would exceed it."
2. Ask if they want to proceed anyway or move an existing card first.
3. Do not move the card until the user confirms.

Never silently exceed a WIP limit.

## Phase Assignment Protocol

Every board has workflow phases that track where a card is in the delivery pipeline. The default phases are: Development, Code Review, QA, Client Review, Ready to Deploy.

### When to Assign Phases

Assign a phase when creating or updating a card if the work state is clear:

- The user says "I'm coding this" or "working on implementation" -> **Development**
- The user opens a PR or asks for a review -> **Code Review**
- The user says "ready for testing" or "needs QA" -> **QA**
- The user says "waiting on the client" or "sent for approval" -> **Client Review**
- The user says "approved", "ready to ship", or "merge it" -> **Ready to Deploy**

If the work state is ambiguous, do not assign a phase. A card without a phase is valid — it simply means the delivery stage is unknown.

### Phase vs Column

Phases and columns serve DIFFERENT purposes:

- **Columns** track the workflow state of the TASK (Backlog, To Do, In Progress, Review, Done)
- **Phases** track the delivery stage of the WORK (Development, Code Review, QA, etc.)

A card can be In Progress (column) during Development (phase), then still In Progress during Code Review (phase). Phases change more frequently than columns.

`update_card` can change both metadata (title, description, priority, phase) AND move to a different column in a single call. When the user mentions a delivery stage change AND a state transition together, use `update_card` with both `phase_name` and `column_name`. Use `move_card` only when you need to reposition within a column or move without any other changes.

### Managing Phases

Use `manage_phases` to list, create, update, delete, or reorder phases on a board:

- `action: "list"` — see all phases on a board (ordered by position)
- `action: "create"` — add a new phase (requires `name`, optional `color` defaults to #00FFFF)
- `action: "update"` — change name or color (requires `phase_id`)
- `action: "delete"` — remove a phase (cards keep their data, phase_id becomes null)
- `action: "reorder"` — reorder phases by providing `ordered_ids` as a JSON array

### Phase Transitions

When you detect a phase change from the conversation, update the card immediately. If the column should also change, include both in the same call:

```
update_card(card_id: "...", phase_name: "Code Review", column_name: "Review")
```

If only the phase changes (column stays the same):

```
update_card(card_id: "...", phase_name: "Code Review")
```

To remove a phase from a card (e.g., the card is no longer in the delivery pipeline):

```
update_card(card_id: "...", unset_phase: true)
```

Do not skip phase transitions without reason. If a card jumps from Development to Ready to Deploy, confirm with the user.

## Card Template

Every card MUST follow this template. No exceptions.

### Title Format

Use the pattern: `[type] short imperative description`

Valid types:
- `[feat]` — new functionality
- `[bug]` — something is broken
- `[chore]` — maintenance, tooling, refactors, dependency updates
- `[spike]` — investigation or proof of concept
- `[docs]` — documentation changes

Examples:
- `[feat] add OAuth2 login flow`
- `[bug] fix null pointer on empty board`
- `[chore] upgrade mcp-go to v0.45`
- `[spike] evaluate Redis vs Memcached for session cache`

The title must be lowercase after the type prefix. Keep it under 60 characters. Use imperative mood ("add", "fix", "update", not "added", "fixing", "updates").

### Description Format

Every description MUST have exactly three sections in this order:

```
## What
One sentence describing what needs to be done.

## Why
What problem this solves or what motivates the work.

## Context
Relevant files, services, endpoints, or technical details.
```

Rules:
- **What**: One clear sentence. No filler, no preamble.
- **Why**: Explain the motivation, not restate the what. "Users can't log in" is good. "Because the login is broken" is restating the title.
- **Context**: File paths, service names, API endpoints, config keys, or any technical detail that helps locate the work. If there is no relevant context yet, write "TBD".

Do not add extra sections. The current state of each card (status, progress, blockers) is tracked via columns, phases, and tags, not in the description.

Do not write vague descriptions like "fix the bug" or "implement the feature". A card description must stand alone without the surrounding chat history.

## Update Protocol

When returning to work on an existing item:

1. Call `get_board` to find the card by searching titles and descriptions
2. If found, use `update_card` to update the title, description, or priority as needed
3. If not found, create a new card
4. Never create a duplicate card for the same work item

If you are unsure whether a card exists, search before creating. A board cluttered with duplicates is worse than a missing card.
