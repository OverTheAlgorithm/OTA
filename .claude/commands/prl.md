## Instructions

Perform the user's requested task in an isolated git worktree.
This command is used in a separate terminal to work in parallel with the main session.

Task: $ARGUMENTS

## Setup

1. Auto-generate a branch name from the task description.
   - Format: `parallel/<type>-<description>` (e.g. `parallel/fix-email-logo`, `parallel/feat-admin-dashboard`)
   - Type: feat, fix, refactor, chore, etc. (conventional commits)

2. Update main and create worktree:
   ```
   git fetch origin && git pull origin main
   git worktree add ../OTA-worktrees/<branch> -b <branch>
   cd ../OTA-worktrees/<branch>
   ```

3. All work happens inside this worktree.

## On completion

1. Commit changes (conventional commit format)
2. Verify build/tests pass
3. Return to original project directory, merge into main:
   ```
   cd <original project path>
   git merge <branch>
   ```
4. Clean up worktree and branch:
   ```
   git worktree remove ../OTA-worktrees/<branch>
   git branch -d <branch>
   ```

## Rules

- Never touch main branch while working in the worktree
- If web/ files are modified, run `npm run build` before merge
- Build/tests must pass before merging
- On merge conflict, report to user and wait for instructions
