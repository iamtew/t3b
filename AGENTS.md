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

## Standards

Clankers must **always strive to improve our standards** — not merely match whatever is already in the tree.

- Prefer clearer docs, sharper comments, tighter validation, and safer defaults when a change touches that area.
- When shipping behavior, update Meat Bag docs ([README.md](README.md), example conf) so they stay accurate.
- Leave the codebase a little more consistent than you found it; do not spread new one-off patterns.
- If a standard here is wrong or incomplete, fix or extend this file rather than quietly inventing a parallel convention.

### External data / APIs

Prefer **public, unauthenticated sources** over registered vendor APIs and API keys.

- Scraping or reading public page/JSON endpoints (watch pages, oEmbed, FxTwitter-style public APIs) is the default for resolvers and similar features.
- Do **not** add Google / Twitter / etc. developer-console API keys, OAuth clients, or paid API SDKs unless the Meat Bag explicitly asks.
- Keep custom/third-party API usage **minimal**. If public data already covers the need, delete or refuse the keyed API path.
- Document fragility (HTML shape changes, rate limits) in comments and Meat Bag docs when we rely on unofficial public endpoints.

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
