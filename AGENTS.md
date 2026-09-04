# AGENTS.md - t3b

This file guides **Clankers** (AI agents and automated editors) working on code the **Meat Bags** (human operators) maintain. 
Meat Bags read it too. Clankers must follow it when changing this repository.

User-facing documentation lives in [README.md](README.md).

---

## Terminology

| Term                         | Meaning                                               |
| ---------------------------- | ----------------------------------------------------- |
| **Clanker** / **Clankers**   | Any AI assistant, agent, or automated editor          |
| **Meat Bag** / **Meat Bags** | Human operators, contributors, and dev-workflow users |

**Rules for Clankers**

- Use **Clanker** / **Clankers** and **Meat Bag** / **Meat Bags** in this file, in new comments where natural, and in commit or PR text when the Meat Bag uses that voice.
- Do not refer to Clankers as "AI", "the assistant", "LLM", or similar in agent-facing docs or comments governed by this file.
- Do not rewrite existing comments or docs solely to swap terminology.
- Always ensure code has comments, for the 'Meat Bags' to understand what we're doing.

---

## Commit messages

Clankers writing commits must follow this shape unless the Meat Bag asks otherwise:

- **Subject:** short, punchy, and allowed to be a little funny. One line. Say *why* this landed, not a dull file inventory.
- **Body:** a bullet list of what the commit actually contains (packages, behaviors, stubs, docs). Keep bullets concrete and skimmable.
- Separate subject and body with a blank line. No trailer spam unless the Meat Bag wants it.

Example:

```
Wire up the bot before the network notices

- Go module, Justfile, and .gitignore
- TOML config load/validate + example conf
- Foreground IRC connect/join/log + Ctrl+C quit
- Hostmask auth helpers; daemon/SASL CLI stubs
```

---
