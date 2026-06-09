#!/usr/bin/env bash
# ===========================================================================
# CPA 指纹版本自动检测
# 检查 Claude Code CLI 和 SDK 的最新版本，与服务器配置对比
# 用法: ./check_versions.sh          — 仅检查
#       ./check_versions.sh --apply  — 检查 + 自动更新服务器配置
# ===========================================================================
set -euo pipefail

REMOTE="duoglas@dm.kuoo.uk"
SSH_PORT=2255
CONFIG="/opt/cliproxyapi/config.yaml"
STATE_FILE="$(dirname "$0")/.version_state.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

# ---- 获取最新版本 ----
echo "Checking npm registry..."
CLI_VERSION=$(curl -sf https://registry.npmjs.org/@anthropic-ai/claude-code/latest | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)
SDK_VERSION=$(curl -sf https://registry.npmjs.org/@anthropic-ai/sdk/latest | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$CLI_VERSION" ] || [ -z "$SDK_VERSION" ]; then
    echo -e "${RED}Failed to fetch versions from npm${NC}"
    exit 1
fi

echo "Latest: claude-code=$CLI_VERSION, sdk=$SDK_VERSION"

# ---- 读取服务器当前配置 ----
CURRENT_UA=$(ssh -p $SSH_PORT $REMOTE "grep 'user-agent:' $CONFIG" 2>/dev/null | grep -o 'claude-cli/[^"]*' | head -1 | cut -d/ -f2 | tr -d ' ')
CURRENT_SDK=$(ssh -p $SSH_PORT $REMOTE "grep 'package-version:' $CONFIG" 2>/dev/null | grep -o '"[^"]*"' | tr -d '"')

echo "Server: claude-code=${CURRENT_UA:-unknown}, sdk=${CURRENT_SDK:-unknown}"

# ---- 对比 ----
NEEDS_UPDATE=false

if [ "$CLI_VERSION" != "$CURRENT_UA" ]; then
    echo -e "${YELLOW}[UPDATE] claude-code: $CURRENT_UA -> $CLI_VERSION${NC}"
    NEEDS_UPDATE=true
else
    echo -e "${GREEN}[OK] claude-code: $CLI_VERSION${NC}"
fi

if [ "$SDK_VERSION" != "$CURRENT_SDK" ]; then
    echo -e "${YELLOW}[UPDATE] sdk: $CURRENT_SDK -> $SDK_VERSION${NC}"
    NEEDS_UPDATE=true
else
    echo -e "${GREEN}[OK] sdk: $SDK_VERSION${NC}"
fi

# ---- 检查上游更新 ----
echo ""
echo "Checking upstream..."
UPSTREAM_SHA=$(curl -sf https://api.github.com/repos/Arron196/CLIProxyAPI/commits/main | grep -o '"sha":"[^"]*"' | head -1 | cut -d'"' -f4 | head -c7)
LOCAL_SHA=$(cd "$(dirname "$0")/.." && git rev-parse upstream/main 2>/dev/null | head -c7 || echo "unknown")
if [ "$UPSTREAM_SHA" != "$LOCAL_SHA" ]; then
    echo -e "${YELLOW}[UPDATE] Arron196 upstream has new commits (remote=$UPSTREAM_SHA, local=$LOCAL_SHA)${NC}"
    echo "  Run: cd my-cpa && git fetch upstream && git merge upstream/main && ./build.sh deploy"
else
    echo -e "${GREEN}[OK] Upstream in sync${NC}"
fi

# ---- 自动应用 ----
if [ "${1:-}" = "--apply" ] && [ "$NEEDS_UPDATE" = true ]; then
    echo ""
    echo "Applying updates to server..."
    ssh -p $SSH_PORT $REMOTE "sudo sed -i \
        -e 's|claude-cli/[0-9.]*|claude-cli/${CLI_VERSION}|' \
        -e 's|package-version: \"[0-9.]*\"|package-version: \"${SDK_VERSION}\"|' \
        $CONFIG && cd /opt/cliproxyapi && sudo docker compose restart" 2>&1
    echo -e "${GREEN}Server config updated and service restarted.${NC}"
fi

# ---- 保存状态 ----
cat > "$STATE_FILE" << EOF
{
  "checked_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "claude_code": "$CLI_VERSION",
  "sdk": "$SDK_VERSION",
  "upstream_sha": "$UPSTREAM_SHA",
  "needs_update": $NEEDS_UPDATE
}
EOF

if [ "$NEEDS_UPDATE" = true ] && [ "${1:-}" != "--apply" ]; then
    echo ""
    echo "To auto-apply: ./check_versions.sh --apply"
fi
