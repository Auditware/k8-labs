#!/bin/sh
# Build, load, and wire up /dev/kuaf in one step.
#
# Why the mknod: the module registers a misc device (you can see it at
# /sys/class/misc/kuaf), but this lab's /dev is the guest node's /dev with no
# udev/devtmpfs auto-population, so the /dev/kuaf node has to be created by hand
# from the major:minor the kernel assigned. Re-run any time; it reloads cleanly.
set -e
cd "$(dirname "$0")"

make >/dev/null
rmmod kuaf 2>/dev/null || true
insmod kuaf.ko

DEV=$(cat /sys/class/misc/kuaf/dev)   # e.g. 10:260
rm -f /dev/kuaf
mknod /dev/kuaf c "${DEV%:*}" "${DEV#*:}"
chmod 666 /dev/kuaf

echo "kuaf loaded -> /dev/kuaf ($DEV). Now: cd /app/fuzz && gcc -O2 -o fuzz fuzz.c && ./fuzz uaf"
