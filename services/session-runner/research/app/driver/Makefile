# Out-of-tree kbuild for the kuaf teaching driver (vuln.c -> kuaf.ko).
#
# Needs the TARGET kernel's build tree. On the lab's research microVM that is the
# KASAN kernel's own headers under /lib/modules/$(uname -r)/build, shipped with
# the kernel (see docs/architecture/kasan-research-lab.md). `make` there produces
# kuaf.ko; `insmod kuaf.ko` loads it and creates /dev/kuaf.
#
# Building against a stock kernel (or the local runc host kernel, which has no
# matching build tree) will not produce a loadable module - that is expected:
# the whole point of the lab is the instrumented research kernel.

obj-m += kuaf.o
kuaf-y := vuln.o

KDIR ?= /lib/modules/$(shell uname -r)/build
PWD  := $(shell pwd)

# KBUILD_MODPOST_WARN=1: the lab kernel ships a modules_prepare build tree (no
# full Module.symvers), so modpost can't confirm the exported symbols the module
# references. The kernel is built with MODVERSIONS off, so that check is advisory
# - the symbols resolve against the running kernel at insmod - and this downgrades
# it from a fatal error to a warning.
all:
	$(MAKE) -C $(KDIR) M=$(PWD) KBUILD_MODPOST_WARN=1 modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean

.PHONY: all clean
