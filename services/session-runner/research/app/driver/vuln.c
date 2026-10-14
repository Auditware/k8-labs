// SPDX-License-Identifier: GPL-2.0
//
// kuaf - a deliberately vulnerable teaching driver for the kernel-driver
// research lab. It exposes a misc character device (/dev/kuaf) with a tiny ioctl
// interface that contains a textbook use-after-free: FREE releases the object
// but leaves the dangling pointer live, so a later USE dereferences freed slab
// memory. Under the lab's KASAN kernel that USE-after-FREE is an immediate,
// loud crash oracle; the exploitation exercise is to turn it into uid 0 before
// KASAN would ever fire, by reclaiming the freed object with one you control.
//
// This is intentionally insecure. It exists only inside the disposable research
// microVM. Do not ship anything like it.
//
// Build against the lab kernel's build tree:  make        (see Makefile)
// Load:                                        insmod kuaf.ko
// Then drive it from userspace (see /app/fuzz/fuzz.c).

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/miscdevice.h>
#include <linux/fs.h>
#include <linux/slab.h>
#include <linux/uaccess.h>

#define KUAF_IOCTL_ALLOC 0x1001 // kmalloc an object, remember it
#define KUAF_IOCTL_USE   0x1002 // call the object's function pointer
#define KUAF_IOCTL_FREE  0x1003 // kfree the object (but keep the pointer!)
#define KUAF_IOCTL_WRITE 0x1004 // copy user bytes INTO the object (the groom lever)

// A small heap object with a function pointer. Its size lands it in a common
// kmalloc cache (kmalloc-64 here), so it shares a slab with plenty of userland-
// controllable objects (msg_msg, setxattr, pipe_buffer) - the reclaim surface.
struct kuaf_obj {
	void (*fn)(void); // called by USE - the control-flow hijack target
	char note[48];    // padding + a place WRITE lands, to inspect grooming
};

// The bug in one variable: a single global dangling pointer. A real driver
// would null this on free and lock around it; this one does neither, on purpose.
static struct kuaf_obj *obj;

// The benign default callback USE invokes before anything is freed - so the
// happy path prints a line and the researcher sees the mechanism before they
// attack it.
static void kuaf_default_fn(void)
{
	pr_info("kuaf: default fn() ran (obj still valid)\n");
}

static long kuaf_ioctl(struct file *f, unsigned int cmd, unsigned long arg)
{
	switch (cmd) {
	case KUAF_IOCTL_ALLOC:
		kfree(obj); // free any previous object first (also UAF-able)
		obj = kmalloc(sizeof(*obj), GFP_KERNEL);
		if (!obj)
			return -ENOMEM;
		obj->fn = kuaf_default_fn;
		memset(obj->note, 0, sizeof(obj->note));
		pr_info("kuaf: allocated obj at %px (fn=%px)\n", obj, obj->fn);
		return 0;

	case KUAF_IOCTL_USE:
		if (!obj)
			return -EINVAL;
		// USE-AFTER-FREE: if FREE ran, obj still points at released slab
		// memory. Calling obj->fn() here dereferences and *invokes* whatever
		// now occupies that slot - the whole exploit hinges on this line.
		obj->fn();
		return 0;

	case KUAF_IOCTL_WRITE: {
		// Copy 56 user bytes over the object. Harmless while the object is
		// live; after FREE it becomes a way to author freed-slab contents if
		// the pointer is still the winner - useful for seeing the layout.
		if (!obj)
			return -EINVAL;
		if (copy_from_user(obj, (void __user *)arg, sizeof(*obj)))
			return -EFAULT;
		return 0;
	}

	case KUAF_IOCTL_FREE:
		if (!obj)
			return -EINVAL;
		kfree(obj);
		// THE BUG: no `obj = NULL;`. The pointer stays live and reusable, so
		// the slot can be reclaimed and the next USE/WRITE lands on it.
		pr_info("kuaf: freed obj (pointer left dangling)\n");
		return 0;

	default:
		return -ENOTTY;
	}
}

static const struct file_operations kuaf_fops = {
	.owner = THIS_MODULE,
	.unlocked_ioctl = kuaf_ioctl,
	.compat_ioctl = kuaf_ioctl,
};

static struct miscdevice kuaf_dev = {
	.minor = MISC_DYNAMIC_MINOR,
	.name = "kuaf",
	.fops = &kuaf_fops,
	.mode = 0666, // world-accessible on purpose: any lab process may open it
};

static int __init kuaf_init(void)
{
	pr_info("kuaf: loaded - /dev/kuaf ready (obj size=%zu)\n", sizeof(struct kuaf_obj));
	return misc_register(&kuaf_dev);
}

static void __exit kuaf_exit(void)
{
	misc_deregister(&kuaf_dev);
	kfree(obj);
	obj = NULL;
	pr_info("kuaf: unloaded\n");
}

module_init(kuaf_init);
module_exit(kuaf_exit);

MODULE_LICENSE("GPL");
MODULE_DESCRIPTION("Deliberately vulnerable UAF teaching driver for the k8-labs kernel research lab");
MODULE_AUTHOR("k8-labs");
