// Package drivers is the research driver catalog: it parses uploaded compiled
// kernel modules (.ko), stores them, and serves the vetting + catalog queries.
// See docs/architecture/research-driver-catalog.md.
//
// The parse here is the load-bearing input check. An uploaded .ko is untrusted
// bytes; we NEVER execute it, insmod it, or trust the client's filename or
// content-type. Everything the rest of the system shows (name, vermagic,
// license, ...) is extracted from the ELF itself with the stdlib debug/elf
// reader, so a hostile upload can at worst be a malformed ELF we reject.
package drivers

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// MaxKoBytes caps an uploaded module. Real out-of-tree modules are tens of KB to
// low single-digit MB; 8 MiB is comfortably above anything legitimate while
// keeping a hostile upload from bloating a Postgres row. Mirrors the hard-cap
// posture of accounts.MaxAvatarBytes.
const MaxKoBytes = 8 * 1024 * 1024

// maxModinfoBytes caps the .modinfo section specifically. Real modules carry a
// few hundred bytes here (a handful of "key=value" pairs); 1 MiB is orders of
// magnitude above anything legitimate. This is a separate limit from MaxKoBytes
// on purpose: MaxKoBytes bounds the bytes that arrive, this bounds what they are
// allowed to expand into. See readModinfo.
const maxModinfoBytes = 1024 * 1024

// sigTrailer is what the kernel's sign-file appends to a signed module. Its
// presence is all we assert about signing - the research kernels do NOT enforce
// module signatures at load (MODVERSIONS off, no CONFIG_MODULE_SIG enforcement),
// so this is provenance for the vetting UI, never a load-time gate.
var sigTrailer = []byte("~Module signature appended~\n")

// ErrNotKo is returned when the bytes are not a loadable kernel module: not an
// ELF, not a relocatable object, or missing the .modinfo section every real .ko
// carries. It maps to a 400 at the API edge.
var ErrNotKo = errors.New("not a valid kernel module (.ko): expected an ELF relocatable object with a .modinfo section")

// ParsedKo is the metadata extracted from a .ko, plus the content hash. Every
// field is derived from the bytes; none comes from the client.
type ParsedKo struct {
	SHA256      string
	Machine     string // ELF machine, e.g. "EM_X86_64" - the arch the module targets
	ModName     string // modinfo "name" (often the module's own name)
	Vermagic    string // modinfo "vermagic" - the real load-compatibility key
	License     string
	Depends     string
	Description string
	Author      string
	SrcVersion  string
	IsSigned    bool
	Signer      string // modinfo "signer", if the signer left one; usually empty
}

// ParseKo validates that data is a loadable kernel module and extracts its
// metadata. It reads only headers and the .modinfo section - it does not and
// must not run the module. Returns ErrNotKo for anything that is not a real .ko.
func ParseKo(data []byte) (ParsedKo, error) {
	if len(data) == 0 {
		return ParsedKo{}, ErrNotKo
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return ParsedKo{}, fmt.Errorf("%w: %v", ErrNotKo, err)
	}
	defer f.Close()

	// A kernel module is a relocatable object (unlinked). Reject executables,
	// shared objects, and core files outright - they are not modules.
	if f.Type != elf.ET_REL {
		return ParsedKo{}, fmt.Errorf("%w: ELF type is %s, want ET_REL", ErrNotKo, f.Type)
	}

	sec := f.Section(".modinfo")
	if sec == nil {
		return ParsedKo{}, ErrNotKo
	}
	raw, err := readModinfo(sec)
	if err != nil {
		return ParsedKo{}, err
	}
	info := parseModinfo(raw)

	sum := sha256.Sum256(data)
	return ParsedKo{
		SHA256:      hex.EncodeToString(sum[:]),
		Machine:     f.Machine.String(),
		ModName:     info["name"],
		Vermagic:    info["vermagic"],
		License:     info["license"],
		Depends:     info["depends"],
		Description: info["description"],
		Author:      info["author"],
		SrcVersion:  info["srcversion"],
		IsSigned:    bytes.HasSuffix(data, sigTrailer),
		Signer:      info["signer"],
	}, nil
}

// readModinfo returns the .modinfo bytes, refusing the shapes an attacker uses
// to turn an 8 MiB upload into a multi-gigabyte allocation.
//
// The hole this closes: Section.Data() is saferio.ReadData(s.Open(), s.Size),
// and for a section flagged SHF_COMPRESSED the stdlib makes Open() a live
// zlib/zstd decompressor while s.Size becomes the size declared in the section's
// own compression header - a number the uploader chooses. saferio reads in
// chunks but keeps appending for the whole declared size, so a few KB of
// compressed zeroes (they pack ~1000:1) with ch_size set to gigabytes makes the
// backend allocate gigabytes. MaxKoBytes bounds the UPLOAD, not what the upload
// expands to, and the backend pod's memory limit is far below that, so a single
// small request OOM-kills the shared process. The upload limits cannot absorb
// the retry either: they count stored rows, and a parse that fails never stores
// one (see EnforceUploadLimits), so the attempt is free and repeatable.
//
// Three guards, cheapest first. A real .modinfo is a few hundred bytes of plain
// NUL-separated "key=value" text emitted by modpost: never compressed, never
// SHT_NOBITS, never anywhere near maxModinfoBytes. Anything else is not a module
// we should be accepting, so rejecting is both safe and honest.
func readModinfo(sec *elf.Section) ([]byte, error) {
	if sec.Flags&elf.SHF_COMPRESSED != 0 {
		return nil, fmt.Errorf("%w: .modinfo is compressed", ErrNotKo)
	}
	// SHT_NOBITS occupies no file bytes; Size is purely declarative, so reading it
	// would be inventing data (and Data() would allocate Size for it).
	if sec.Type == elf.SHT_NOBITS {
		return nil, fmt.Errorf("%w: .modinfo has no contents (SHT_NOBITS)", ErrNotKo)
	}
	if sec.Size > maxModinfoBytes {
		return nil, fmt.Errorf("%w: .modinfo is %d bytes, over the %d limit", ErrNotKo, sec.Size, maxModinfoBytes)
	}
	// Read through a LimitReader rather than Data() so the ceiling holds even if a
	// future stdlib change makes Size disagree with what the reader produces.
	raw, err := io.ReadAll(io.LimitReader(sec.Open(), maxModinfoBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: reading .modinfo: %v", ErrNotKo, err)
	}
	return raw, nil
}

// parseModinfo splits a .modinfo blob - a run of NUL-terminated "key=value"
// strings - into a map. Later duplicate keys win, matching how the kernel reads
// them. Entries without an '=' are ignored.
func parseModinfo(raw []byte) map[string]string {
	out := map[string]string{}
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		k, v, ok := strings.Cut(string(entry), "=")
		if !ok {
			continue
		}
		out[k] = v
	}
	return out
}
