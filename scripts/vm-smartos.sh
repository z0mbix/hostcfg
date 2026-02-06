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

echo "Booting SmartOS (live image)..."
exec qemu-system-x86_64 \
  -m "${MEMORY}" \
  -smp 2 \
  -cdrom "${ISO}" \
  -boot d \
  -drive "file=${ZONES_DISK},format=qcow2" \
  -netdev "user,id=net0,hostfwd=tcp::${SSH_PORT}-:22" \
  -device virtio-net-pci,netdev=net0 \
  -display none \
  -serial mon:stdio \
  -nographic
