#!/usr/bin/env bash
#
# Start an OmniOS VM in QEMU for testing hostcfg.
#
# Prerequisites:
#   brew install qemu
#
# Usage:
#   ./scripts/vm-omnios.sh          # Boot from existing disk (or install ISO if no disk)
#   ./scripts/vm-omnios.sh install  # Force boot from ISO for fresh install
#
# After boot, connect to the console via VNC:
#   open vnc://localhost:5900
#
# After install, the VM is accessible via SSH:
#   ssh -p 2222 root@localhost
#
# To copy the hostcfg binary into the VM:
#   CGO_ENABLED=0 GOOS=illumos GOARCH=amd64 go build -o hostcfg-illumos ./cmd/hostcfg
#   scp -P 2222 hostcfg-illumos root@localhost:/usr/local/bin/hostcfg

set -euo pipefail

VM_DIR="${HOME}/.local/share/hostcfg-vms/omnios"
DISK="${VM_DIR}/disk.qcow2"
ISO="${VM_DIR}/omnios.iso"
ISO_URL="https://downloads.omnios.org/media/stable/omnios-r151056.iso"
DISK_SIZE="20G"
MEMORY="2048"
SSH_PORT="2222"
VNC_DISPLAY="0"

mkdir -p "${VM_DIR}"

# Download ISO if not present
if [[ ! -f "${ISO}" ]]; then
  echo "Downloading OmniOS ISO..."
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
echo "QEMU monitor on stdio (type 'quit' to stop VM)"
echo ""

# Use a monitor socket to set the VNC password, then hand control to stdio
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

# Set VNC password via monitor socket
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

echo "VM started in background (PID in QEMU)"
echo "To stop: python3 -c \"import socket; s=socket.socket(socket.AF_UNIX); s.connect('${MONITOR_SOCK}'); s.sendall(b'quit\n'); s.close()\""
