// fuzz - a starter ioctl fuzzer for /dev/kuaf, the lab's teaching driver.
//
// It is deliberately small: open the device and hammer its ioctl surface with a
// mix of the known commands and random ones, so you can watch (in `strace`, in
// `dmesg`, and in the Defender pane) how the driver reacts - and, on the KASAN
// research kernel, trip the use-after-free the driver contains. This is a
// STARTING POINT, not a finished exploit: the interesting work is turning the
// crash into control by grooming the freed slot.
//
// Build:  gcc -O2 -o fuzz fuzz.c
// Run:    ./fuzz            (a few thousand random ops)
//         ./fuzz uaf        (the minimal alloc->free->use sequence that trips
//                            KASAN - the oracle you build the exploit around)

#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#include <sys/ioctl.h>

#define KUAF_IOCTL_ALLOC 0x1001
#define KUAF_IOCTL_USE   0x1002
#define KUAF_IOCTL_FREE  0x1003
#define KUAF_IOCTL_WRITE 0x1004

// Random-mode commands. KUAF_IOCTL_USE is deliberately left OUT: it calls the
// object's function pointer, which on a randomly-freed slot jumps into poison and
// takes the microVM down - unfriendly for a hammer loop. The `uaf` demo trips the
// bug intentionally (with a WRITE) instead. Grooming + USE is the exploit, done
// by hand once you understand the layout.
static const unsigned int cmds[] = {
	KUAF_IOCTL_ALLOC, KUAF_IOCTL_FREE, KUAF_IOCTL_WRITE,
};

// The minimal sequence that demonstrates the bug: allocate an object, free it
// (the driver leaves the pointer dangling), then touch it. On the KASAN kernel
// that touch is a use-after-free and KASAN prints a report naming the object -
// that report is your oracle. We touch it with a WRITE (KUAF_IOCTL_WRITE), which
// KASAN catches WITHOUT transferring control, so the report lands and the box
// stays up for you to read `dmesg`. (KUAF_IOCTL_USE also trips KASAN, but it then
// CALLS the dangling function pointer - on an ungroomed slot that jumps into
// poison and takes the microVM down, so it is the wrong move for just seeing the
// oracle. Weaponizing is the reverse: groom the freed slot with a controlled
// pointer, THEN USE, so the call goes where you choose.)
static int demo_uaf(int fd)
{
	unsigned char buf[56];
	memset(buf, 0x41, sizeof(buf));
	printf("[*] ALLOC\n");
	if (ioctl(fd, KUAF_IOCTL_ALLOC, 0) < 0) { perror("alloc"); return 1; }
	printf("[*] FREE   (pointer left dangling by the driver)\n");
	if (ioctl(fd, KUAF_IOCTL_FREE, 0) < 0) { perror("free"); return 1; }
	printf("[*] WRITE  (<- use-after-free write; KASAN fires here on the research kernel)\n");
	ioctl(fd, KUAF_IOCTL_WRITE, buf);
	printf("[*] done - check `dmesg` for the KASAN use-after-free report (your oracle).\n");
	printf("    on a non-KASAN kernel this write is silent; that is the point of the lab kernel.\n");
	return 0;
}

static void fuzz(int fd, long iters)
{
	unsigned char buf[64];
	for (long i = 0; i < iters; i++) {
		unsigned int cmd = cmds[rand() % (sizeof(cmds) / sizeof(cmds[0]))];
		// For WRITE, hand a user buffer full of a recognizable byte so you can
		// spot it if it lands in a freed slot; other commands ignore arg.
		memset(buf, 0x41 + (i & 0x1f), sizeof(buf));
		ioctl(fd, cmd, buf);
		if ((i & 0x3ff) == 0)
			printf("[.] %ld ops\n", i);
	}
	printf("[+] done: %ld random ops\n", iters);
}

int main(int argc, char **argv)
{
	srand((unsigned)time(NULL) ^ (unsigned)getpid());

	int fd = open("/dev/kuaf", O_RDWR);
	if (fd < 0) {
		perror("open /dev/kuaf");
		fprintf(stderr, "is the driver loaded? build it in /app/driver and `insmod kuaf.ko`\n");
		return 1;
	}

	if (argc > 1 && strcmp(argv[1], "uaf") == 0)
		return demo_uaf(fd);

	long iters = (argc > 1) ? atol(argv[1]) : 4000;
	fuzz(fd, iters);
	close(fd);
	return 0;
}
