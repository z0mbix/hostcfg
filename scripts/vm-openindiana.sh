#!/usr/bin/env bash
#
# Start an OpenIndiana VM in QEMU for testing hostcfg.
#
# Prerequisites:
#   brew install qemu
#
# Usage:
#   ./scripts/vm-openindiana.sh          # Boot from existing disk
#   ./scripts/vm-openindiana.sh install  # Force boot from ISO for fresh install
#
# After boot, connect to the console via VNC:
#   open vnc://localhost:5902
#
# After install, the VM is accessible via SSH:
#   ssh -p 2224 <user>@localhost
#
# To copy the hostcfg binary into the VM:
#   CGO_ENABLED=0 GOOS=illumos GOARCH=amd64 go build -o hostcfg-illumos ./cmd/hostcfg
#   scp -P 2224 hostcfg-illumos <user>@localhost:~/hostcfg

set -euo pipefail

VM_DIR="${HOME}/.local/share/hostcfg-vms/openindiana"
DISK="${VM_DIR}/disk.qcow2"
ISO="${VM_DIR}/openindiana.iso"
ISO_URL="https://dlc.openindiana.org/isos/hipster/20251026/OI-hipster-text-20251026.iso"
DISK_SIZE="20G"
MEMORY="2048"
SSH_PORT="2224"
VNC_DISPLAY="2"

mkdir -p "${VM_DIR}"

# Download ISO if not present
if [[ ! -f "${ISO}" ]]; then
  echo "Downloading OpenIndiana ISO..."
  curl -L -o "${ISO}" "${ISO_URL}"
fi

# Create disk if not present
if [[ ! -f "${DISK}" ]]; then
  echo "Creating ${DISK_SIZE} disk image..."
  qemu-img create -f qcow2 "${DISK}" "${DISK_SIZE}"
fi

BOOT_ARGS=()
if [[ "${1:-}" == "install" ]] || [[ ! -s "${DISK}" ]] || [[ $(stat -f%z "${DISK}" 2>/dev/null || stat -c%s "${DISK}" 2>/dev/null) -lt 1000000 ]]; then
  echo "Booting from ISO (install mode)..."
  BOOT_ARGS+=(-drive "file=${ISO},media=cdrom" -boot d)
else
  echo "Booting from disk..."
  BOOT_ARGS+=(-boot c)
fi

VNC_PASSWORD="hostcfg"

echo "VNC console: vnc://localhost:$((5900 + VNC_DISPLAY)) (password: ${VNC_PASSWORD})"
echo ""

MONITOR_SOCK="${VM_DIR}/monitor.sock"
rm -f "${MONITOR_SOCK}"

qemu-system-x86_64 \
  -m "${MEMORY}" \
  -smp 2 \
  -cpu qemu64 \
  -machine q35 \
  -drive "file=${DISK},format=qcow2,if=virtio" \
  "${BOOT_ARGS[@]}" \
  -netdev "user,id=net0,hostfwd=tcp::${SSH_PORT}-:22" \
  -device virtio-net-pci,netdev=net0 \
  -display "vnc=:${VNC_DISPLAY},password=on" \
  -vga std \
  -monitor "unix:${MONITOR_SOCK},server,nowait" \
  -daemonize

sleep 1
python3 -c "
import socket, time
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect('${MONITOR_SOCK}')
time.sleep(0.5)
s.recv(1024)
s.sendall(b'change vnc password ${VNC_PASSWORD}\n')
time.sleep(0.5)
s.close()
"

echo "VM started in background"
echo "To stop: python3 -c \"import socket; s=socket.socket(socket.AF_UNIX); s.connect('${MONITOR_SOCK}'); s.sendall(b'quit\n'); s.close()\""
