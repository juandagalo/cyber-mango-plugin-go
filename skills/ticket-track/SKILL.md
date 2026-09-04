---
name: ticket-track
description: Ticket tracking workflow — sync external tickets (GitHub issues, Jira, Linear) with the Cyber Mango kanban board.
---

# Ticket Tracking Protocol

When the user mentions an external ticket, issue, or work item from any issue tracker, follow this protocol to sync it with the Cyber Mango board and keep both systems in agreement.

## Trigger Conditions

Activate this protocol when the user references:
- A GitHub issue URL (e.g., `https://github.com/org/repo/issues/123`)
- A GitHub issue number with context (e.g., "issue #42", "GH-42")
- A Jira ticket ID (e.g., `PROJ-100`, `JIRA-55`)
- A Linear issue ID (e.g., `ENG-12`, `CYB-7`)
- Any other external work item from an issue tracker (Linear, Shortcut, Azure DevOps, etc.)

Also activate when the user says things like "let's work on the ticket", "check the issue", or "I'm picking up [external reference]".

## Step-by-Step Workflow

### 1. Parse the Ticket Reference

Extract from the user's message:
- **Source system**: GitHub, Jira, Linear, etc.
- **Ticket ID**: The unique identifier (e.g., `123`, `PROJ-100`)
- **URL**: If provided, preserve it exactly
- **Title/description**: If visible in context or inferable from the URL

### 2. Find the Card

Search engram by ticket id, then `get_board_summary` to confirm; report column, priority and tags. If a prior session saved a ticket-to-card mapping, retrieve it via `mem_get_observation`. On the board, tracked tickets carry the `[SOURCE-ID]` prefix at the start of the title.

If a card is found via engram mapping but not in the board, assume it was deleted and create a new one.

### 3a. If No Card Exists — Create It

Call `create_card` with:
- **Title**: `[SOURCE-ID] Brief description` — see naming convention below
- **Column**: Place the card in the column whose `description` matches the ticket's state (see board-manage); the first column by position is the default when nothing fits yet.
- **Priority**: medium (default) unless the ticket signals urgency
- **Description**: Use the standard What/Why/Context card; put the ticket URL, external id and acceptance criteria inside `## Context`.

After creating, save the ticket-to-card mapping to engram immediately:
```
mem_save:
  title: "Linked [SOURCE-ID] to card [card_id]"
  type: decision
  topic_key: ticket-map/[SOURCE-ID]
  content:
    What: Created card for [SOURCE-ID] on the Cyber Mango board
    Why: User referenced this ticket in session
    Where: card_id=[card_id], board card title=[card title]
    Learned: [any non-obvious detail about the ticket]
```

### 3b. If a Card Already Exists — Update It

Call `update_card` if any of the following have changed:
- The title needs correction or improvement
- The description can be enriched with new information from the ticket
- The priority should change based on new context
- The column no longer matches the ticket's state (pass `column_name` in the same call)

Do not call `update_card` if nothing has changed. Every card, column, phase and tag write is logged and summarised at session stop; batch related changes.

## Naming Convention

Card titles MUST follow this exact format:

```
[SOURCE-ID] Brief description
```

Examples:
- `[GH-42] Add OAuth2 support to login flow`
- `[JIRA-100] Migrate user table to new schema`
- `[ENG-12] Implement rate limiting on API gateway`

Rules:
- The source prefix is always uppercase, always in square brackets
- Use GitHub -> `GH`, Jira -> the project key (e.g., `JIRA`, `PROJ`), Linear -> the team key (e.g., `ENG`)
- The description after the prefix is sentence-case, concise, action-oriented
- Do not include the full URL in the title — put it in `## Context`

## Cross-Session Recall

Engram is the source of truth for ticket-to-card mappings across sessions. The board reflects current state. These two systems complement each other:

- **Engram**: remembers WHICH card belongs to WHICH ticket, session history, and context
- **Board**: reflects CURRENT state (column, priority, tags) of the work

When saving to engram, use `topic_key: ticket-map/[SOURCE-ID]` as a stable identifier so subsequent sessions can find it with a targeted search.

## Definition of Done

A tracked ticket is complete when ALL of the following are true:

1. The card is in the terminal column (last by position) on the Cyber Mango board
2. All acceptance criteria from the original ticket have been addressed (documented in `## Context`)
3. The ticket in the external system is closed or resolved (the user confirms this)

Do not move a ticket to the terminal column unilaterally based on code being merged. Confirm with the user that the ticket's acceptance criteria are met before moving.

## Status Requests

When the user asks for the status of a tracked ticket, run step 2 (Find the Card) and report from the board, never from memory alone. If the card's state does not match what the user expects, offer to move or update it.
