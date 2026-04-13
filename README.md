# CLIProxyAPI (Anti-Detection Hardened Fork)

English | [中文](README_CN.md)

This repository is a hardened derivative of [`Arron196/CLIProxyAPI`](https://github.com/Arron196/CLIProxyAPI), focused on **anti-detection improvements** for safer daily usage.

## What This Fork Adds

All changes from `Arron196/CLIProxyAPI` are preserved. On top of that, this fork applies the following anti-detection hardening:

| Change | Purpose |
|--------|---------|
| Haiku model cloaking fix | All models now receive full cloaking (billing header + system prompt injection). Previously `claude-3-5-haiku` was skipped, creating a detectable fingerprint. |
| Configurable billing header version | `cc_version` in the billing header now reads from `claude-header-defaults.user-agent` in config, instead of hardcoded `2.1.63`. Update config when Claude Code releases new versions. |
| Remove `Connection: keep-alive` | HTTP/2 hop-by-hop header that real Node.js clients don't send. Removed to eliminate a proxy fingerprint. |
| Random credential selection | Replaced strict round-robin with random selection. Prevents upstream from predicting the next credential based on rotation pattern. |
| Per-credential connection pool | Each credential gets an isolated HTTP transport. Prevents HTTP/2 multiplexing from correlating multiple accounts on the same TCP connection. |
| TOCTOU race fix in transport cache | Prevents duplicate transport creation under concurrent access. |

## Quick Start

Build locally and deploy:

```bash
docker build -t cpa-hardened:latest .
```

Or with Compose:

```bash
docker compose up -d
```

## Configuration

Add `claude-header-defaults` to your `config.yaml` to control the Claude Code fingerprint:

```yaml
claude-header-defaults:
  user-agent: "claude-cli/2.1.63 (external, cli)"
  package-version: "0.74.0"
  runtime-version: "v24.3.0"
  timeout: "600"
  os: "MacOS"          # Options: MacOS, Windows, Linux, FreeBSD
  arch: "arm64"        # Options: arm64, x64, x86
```

When Claude Code releases a new version, update these values — no code changes or rebuild needed.

## Syncing with Upstream

```bash
git remote add upstream https://github.com/Arron196/CLIProxyAPI.git
git fetch upstream
git merge upstream/main
```

## Local Documentation

- SDK usage: [docs/sdk-usage.md](docs/sdk-usage.md)
- SDK advanced topics: [docs/sdk-advanced.md](docs/sdk-advanced.md)
- SDK access/auth: [docs/sdk-access.md](docs/sdk-access.md)
- SDK watcher integration: [docs/sdk-watcher.md](docs/sdk-watcher.md)

## Project Identity

- Upstream base: [`Arron196/CLIProxyAPI`](https://github.com/Arron196/CLIProxyAPI)
- Root upstream: `router-for-me/CLIProxyAPI`
- This fork: independent hardened derivative

## License

MIT. See [LICENSE](LICENSE).
