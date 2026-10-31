# antmove agent plugin

This plugin installs the antmove skill for AI assistants. It teaches agents to use the CLI conservatively: scan first, check disks, preview moves, explain the change, and wait for explicit user approval.

## Install the skill

Install from the public repository:

- Skills CLI: <code>npx skills add MakStashkevich/antmove -a codex claude-code cursor -g -y</code>
- Codex plugin: <code>codex plugin marketplace add MakStashkevich/antmove</code>, then <code>codex plugin add antmove@antmove</code>
- Claude Code: run <code>/plugin marketplace add MakStashkevich/antmove</code>, then <code>/plugin install antmove@antmove</code>.

Repository: https://github.com/MakStashkevich/antmove

The plugin requires the antmove CLI on the user's Mac. Use the installation instructions in the root README. Installing this plugin or the CLI does not authorize an agent to scan, move, restore, or uninstall user data.

The canonical skill is <code>skills/antmove/SKILL.md</code>. This plugin contains a copy because some plugin installers do not preserve repository symlinks.

