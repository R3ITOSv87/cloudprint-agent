#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/R3ITOSv87/cloudprint-agent"
VERSION="latest"
INSTALL_DIR="/usr/local/bin"
BINARY="cloudprint-agent"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l)  ARCH="arm"   ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

FILENAME="${BINARY}_${OS}_${ARCH}"
if [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="${REPO}/releases/latest/download/${FILENAME}"
else
    DOWNLOAD_URL="${REPO}/releases/download/${VERSION}/${FILENAME}"
fi

echo "CloudPrint Agent Installer"
echo "OS: ${OS}, Arch: ${ARCH}"
echo ""
echo "Downloading ${FILENAME}..."
curl -sSL "$DOWNLOAD_URL" -o "/tmp/${BINARY}"
chmod +x "/tmp/${BINARY}"

if [ "$(id -u)" -eq 0 ]; then
    mv "/tmp/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    echo "Installed to ${INSTALL_DIR}/${BINARY}"
else
    DEST="$HOME/.local/bin/${BINARY}"
    mkdir -p "$HOME/.local/bin"
    mv "/tmp/${BINARY}" "$DEST"
    echo "Installed to ${DEST}"
    echo "Make sure \$HOME/.local/bin is in your PATH"
fi

echo ""
echo "Installation complete!"
echo ""
echo "Next: register your agent:"
echo "  cloudprint-agent register --token YOUR_TOKEN --name 'Your Location' --api-url YOUR_API_URL"
