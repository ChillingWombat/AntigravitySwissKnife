# GitHub Workspace

[Features](Features.md) / GitHub Workspace

The GitHub Workspace embeds full-lifecycle issue tracking, pull request management, and autonomous subagent execution monitoring directly into the Antigravity developer environment.

---

## 1. Embedded Kanban Board

The core of the GitHub Workspace is an interactive Kanban board that bridges remote GitHub project management with local development branches:
- **Configurable Workflow Columns**: Organizes work items into five default stages: `Backlog`, `Ready`, `In Progress`, `In Review`, and `Done`.
- **Drag-and-Drop Column Reassignment**: Moving an issue or pull request card across columns updates its local status and synchronizes labels with upstream GitHub repositories.
- **Unified Card Representation**: Renders GitHub issues, pull requests, and local agent tasks with status badges, assignee avatars, branch names, and label tags.

---

## 2. Autonomous Agent Task Tracking

When Antigravity spawns background subagents to tackle specific programming tasks, the GitHub Workspace tracks execution progress in real time:
- **Agent Task Mapping**: Associates individual subagent execution threads directly with corresponding GitHub issue numbers or branch names.
- **Active vs Completed States**: Filters the board to show actively running subagent threads or historical runs.
- **Direct Conversation Deep-Linking**: Clicking an agent card immediately switches the main stage to the exact Antigravity conversation thread driving that subagent's execution, closing the gap between task management and prompt interaction.

---

## 3. Direct Repository Operations

Developers can manage repository workflows without switching to a web browser:
- **Issue Creation & Updates**: Creates new issues with titles, markdown descriptions, milestone assignments, and tags directly from the workspace header.
- **Comment Posting**: In-line discussion thread viewer allowing developers to post comments and feedback on active pull requests.
- **Branch Management**: Displays branch names associated with each card, facilitating fast checkouts and git diff inspections.
- **Context Menu Actions**: Right-clicking cards provides one-click shortcuts to copy branch names, reassign column status, navigate to conversation threads, or open the item on GitHub in the default browser.

---

## 4. Local Persistence & Synchronization Pipeline

All board states, column mappings, and cached issue metadata are stored in `pkg/github` daemon state files (`github_workspace.json`). Changes made offline are queued and synchronized automatically as soon as GitHub API connectivity is verified.
