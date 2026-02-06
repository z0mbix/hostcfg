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
ISO_URL="https://downloads.omnios.org/media/omnios-r151056.iso"
DISK_SIZE="20G"
MEMORY="2048"
SSH_PORT="2222"

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
  BOOT_ARGS+=(-cdrom "${ISO}" -boot d)
else
  echo "Booting from disk..."
  BOOT_ARGS+=(-boot c)
fi

exec qemu-system-x86_64 \
  -m "${MEMORY}" \
  -smp 2 \
  -drive "file=${DISK},format=qcow2" \
  "${BOOT_ARGS[@]}" \
  -netdev "user,id=net0,hostfwd=tcp::${SSH_PORT}-:22" \
  -device virtio-net-pci,netdev=net0 \
  -display none \
  -serial mon:stdio \
  -nographic
