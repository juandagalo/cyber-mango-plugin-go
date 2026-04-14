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

### 2. Search Engram for Prior Mapping

Before touching the board, call `mem_search` with the ticket ID as the query. If a prior session saved a mapping between this ticket and a card ID, retrieve it via `mem_get_observation` to get the full entry. This prevents duplicate work and recovers cross-session context.

### 3. Check the Board

Call `get_board` and scan the card titles for the ticket reference. Look for the naming pattern `[SOURCE-ID]` at the start of card titles (e.g., `[GH-123]`, `[JIRA-100]`). This is the canonical way to identify tracked tickets on the board.

If a card is found via engram mapping but not in the board, assume it was deleted and create a new one.

### 4a. If No Card Exists — Create It

Call `create_card` with:
- **Title**: `[SOURCE-ID] Brief description` — see naming convention below
- **Column**: Backlog (default) unless the user's current intent places it elsewhere
- **Priority**: medium (default) unless the ticket signals urgency
- **Description**: Use the description template below

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

### 4b. If a Card Already Exists — Update It

Call `update_card` if any of the following have changed:
- The title needs correction or improvement
- The description can be enriched with new information from the ticket
- The priority should change based on new context

Do not call `update_card` if nothing has changed — unnecessary updates pollute the activity log.

### 5. Move the Card to Match Current Status

After creating or finding the card, move it to the column that reflects the actual work state:
- If the user is about to start working on it: move to **In Progress**
- If it is waiting on review or a dependency: move to **Review**
- If it is just being tracked for future work: leave in **Backlog** or **To Do**
- If it is already done: move to **Done**

## Naming Convention

Card titles MUST follow this exact format:

```
[SOURCE-ID] Brief description
```

Examples:
- `[GH-42] Add OAuth2 support to login flow`
- `[GH-123] Fix memory leak in WebSocket handler`
- `[JIRA-100] Migrate user table to new schema`
- `[ENG-12] Implement rate limiting on API gateway`
- `[PROJ-55] Remove deprecated payment provider`

Rules:
- The source prefix is always uppercase, always in square brackets
- Use GitHub -> `GH`, Jira -> the project key (e.g., `JIRA`, `PROJ`), Linear -> the team key (e.g., `ENG`)
- The description after the prefix is sentence-case, concise, action-oriented
- Do not include the full URL in the title — put it in the description

## Card Description Template

Use this structure for every tracked ticket card:

```
Source: [Full URL to the original ticket]

Summary:
[1-3 sentence summary of what the ticket is about. Do not just copy the title — add context.]

Acceptance Criteria:
- [Criterion 1, if known]
- [Criterion 2, if known]
- (Add more as they become known)

Notes:
[Any additional context, linked PRs, blocked by, dependencies — update this as work progresses]
```

If acceptance criteria are not available, write "(Not specified — to be confirmed)" rather than leaving the section empty.

## Cross-Session Recall

Engram is the source of truth for ticket-to-card mappings across sessions. The board reflects current state. These two systems complement each other:

- **Engram**: remembers WHICH card belongs to WHICH ticket, session history, and context
- **Board**: reflects CURRENT state (column, priority, tags) of the work

On any session where a ticket is mentioned, search engram FIRST before touching the board. This prevents duplicate cards and recovers context that is not visible in card titles alone.

When saving to engram, use `topic_key: ticket-map/[SOURCE-ID]` as a stable identifier so subsequent sessions can find it with a targeted search.

## Definition of Done

A tracked ticket is complete when ALL of the following are true:

1. The card is in the **Done** column on the Cyber Mango board
2. All acceptance criteria from the original ticket have been addressed (documented in the card description)
3. The ticket in the external system is closed or resolved (the user confirms this)

Do not move a ticket to Done unilaterally based on code being merged. Confirm with the user that the ticket's acceptance criteria are met before moving.

## Sync Protocol

When the user asks for the status of a tracked ticket:

1. Search engram for the ticket mapping to get the card ID
2. Call `get_board` to read the current state of the card
3. Report: current column, priority, tags, and any notes in the description
4. If the card's state does not match what the user expects, offer to move or update it

Do not report status from memory alone — always read the board to get the current state.
