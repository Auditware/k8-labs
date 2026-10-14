/* cgns-exec: put the nested k3s in its own cgroup namespace, then exec it.
 *
 * Under runc (local k3d) the runner pod shares the HOST cgroup namespace, so
 * the inner kubelet sees the outer cluster's kubepods and fights over them
 * (containerd gets killed). Docker fixes this by defaulting new containers to a
 * private cgroup namespace on cgroup v2; Kubernetes exposes no such field, and
 * the busybox in the k3s image has no `unshare -C`. So we do it ourselves:
 * unshare CLONE_NEWCGROUP (+ a private mount ns) and remount cgroup2 so the
 * inner k3s sees only its own, empty cgroup root.
 *
 * In prod this is a no-op that never runs the interesting path: each Kata
 * microVM already has its own kernel and cgroup root. This helper only exists
 * to reproduce that isolation locally, keeping the image identical either way.
 *
 * Usage: cgns-exec <cmd> [args...]
 */
#define _GNU_SOURCE
#include <sched.h>
#include <sys/mount.h>
#include <unistd.h>
#include <stdio.h>
#include <errno.h>
#include <string.h>

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "cgns-exec: usage: cgns-exec <cmd> [args...]\n");
        return 2;
    }
    if (unshare(CLONE_NEWCGROUP | CLONE_NEWNS) != 0) {
        fprintf(stderr, "cgns-exec: unshare: %s\n", strerror(errno));
        return 1;
    }
    /* Don't let our cgroup remount propagate back to the host mount tree. */
    if (mount(NULL, "/", NULL, MS_REC | MS_PRIVATE, NULL) != 0) {
        fprintf(stderr, "cgns-exec: make-rprivate: %s\n", strerror(errno));
    }
    /* Re-expose cgroup2 so its root is the new namespace's root (our own leaf),
     * not the host's. MNT_DETACH tolerates it being busy. */
    umount2("/sys/fs/cgroup", MNT_DETACH);
    if (mount("cgroup2", "/sys/fs/cgroup", "cgroup2", 0, NULL) != 0) {
        fprintf(stderr, "cgns-exec: mount cgroup2: %s\n", strerror(errno));
        return 1;
    }
    execvp(argv[1], &argv[1]);
    fprintf(stderr, "cgns-exec: exec %s: %s\n", argv[1], strerror(errno));
    return 127;
}
