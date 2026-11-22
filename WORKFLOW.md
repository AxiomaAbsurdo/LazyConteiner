# Development workflow

Use small, locally testable iterations. Preserve the design in `elements-wireframe.md`; verify inferred syntax against the pinned Exact source before using it.

## Standing skill preferences

- Apply [Karpathy guidelines](https://github.com/multica-ai/andrej-karpathy-skills) to coding: understand first, reuse, make surgical changes, and verify behavior.
- Select task-relevant installed skills from [Matt Pocock's collection](https://github.com/mattpocock/skills).
- Use [Ralph Plan](https://github.com/mastra-ai/mastra) to define concrete steps and acceptance checks for multi-step work.
- Consult [Firstmate](https://github.com/kunchenguid/firstmate) at `/home/axioma/firstmate` for crew operations. Native Codex worker coordination is not a Firstmate shell backend.
- Use [codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) for structural discovery. Check coverage before relying on graph results; read source when coverage is incomplete.
- Use `/home/axioma/obscura` for applicable browser checks. Native Exact tests remain the primary verification for this application.
- Use the installed `typesafe` and `typesafe-ai` skills for structured semantic decisions. Never commit credentials or treat model confidence as test evidence.
- Use `debian-linux-triage` from `github/awesome-copilot` and `debian-ubuntu` from `iuliandita/skills` for Debian packaging and host troubleshooting. Inspect the target release, simulate APT changes, and verify package behavior before installation.

The first iteration used five worker tasks in waves, limited to three concurrent workers plus the supervisor. `plan.md` records scope and acceptance.
