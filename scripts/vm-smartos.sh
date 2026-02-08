#!/usr/bin/env bash
#
# Start a SmartOS VM in QEMU for testing hostcfg.
#
# Prerequisites:
#   brew install qemu
#
# Usage:
#   ./scripts/vm-smartos.sh
#
# SmartOS boots from ISO every time (live image). Configuration is stored on
# the zones disk which persists across reboots.
#
# After boot, connect to the console via VNC:
#   open vnc://localhost:5901
#
# After boot, the VM is accessible via SSH:
#   ssh -p 2223 root@localhost
#
# To copy the hostcfg binary into the VM:
#   CGO_ENABLED=0 GOOS=illumos GOARCH=amd64 go build -o hostcfg-illumos ./cmd/hostcfg
#   scp -P 2223 hostcfg-illumos root@localhost:/opt/custom/bin/hostcfg

set -euo pipefail

VM_DIR="${HOME}/.local/share/hostcfg-vms/smartos"
ZONES_DISK="${VM_DIR}/zones.qcow2"
ISO="${VM_DIR}/smartos.iso"
ISO_URL="https://us-central.manta.mnx.io/Joyent_Dev/public/SmartOS/smartos-latest.iso"
ZONES_DISK_SIZE="20G"
MEMORY="4096"
SSH_PORT="2223"
VNC_DISPLAY="1"

mkdir -p "${VM_DIR}"

# Download ISO if not present
if [[ ! -f "${ISO}" ]]; then
  echo "Downloading SmartOS ISO..."
  curl -L -o "${ISO}" "${ISO_URL}"
fi

# Create zones disk if not present
if [[ ! -f "${ZONES_DISK}" ]]; then
  echo "Creating ${ZONES_DISK_SIZE} zones disk image..."
  qemu-img create -f qcow2 "${ZONES_DISK}" "${ZONES_DISK_SIZE}"
fi

VNC_PASSWORD="hostcfg"

echo "Booting SmartOS (live image)..."
echo "VNC console: vnc://localhost:$((5900 + VNC_DISPLAY)) (password: ${VNC_PASSWORD})"
echo ""

MONITOR_SOCK="${VM_DIR}/monitor.sock"
rm -f "${MONITOR_SOCK}"

qemu-system-x86_64 \
  -m "${MEMORY}" \
  -smp 2 \
  -cpu qemu64 \
  -machine q35 \
  -drive "file=${ISO},media=cdrom" \
  -boot d \
  -drive "file=${ZONES_DISK},format=qcow2,if=virtio" \
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
