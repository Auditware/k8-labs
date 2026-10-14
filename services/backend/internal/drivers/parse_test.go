package drivers

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// buildTestKo hand-crafts a minimal but valid 64-bit little-endian ELF
// relocatable object carrying a .modinfo section, so ParseKo can be exercised
// without a real toolchain. modinfo is written as NUL-terminated key=value
// entries, exactly as the kernel lays them out. etype lets a test forge a
// non-relocatable ELF to prove it is rejected.
func buildTestKo(t *testing.T, etype uint16, modinfo map[string]string, includeModinfo bool) []byte {
	t.Helper()

	// .modinfo blob: "k=v\0" for each entry, deterministic order for the test.
	var mi bytes.Buffer
	for k, v := range modinfo {
		mi.WriteString(k + "=" + v)
		mi.WriteByte(0)
	}
	modinfoData := mi.Bytes()

	// Section header string table.
	shstrtab := append([]byte{0}, []byte(".modinfo\x00.shstrtab\x00")...)
	nameModinfo := uint32(1)   // offset of ".modinfo" in shstrtab
	nameShstrtab := uint32(10) // offset of ".shstrtab"

	const ehSize = 64
	const shSize = 64
	shnum := uint16(3)
	if !includeModinfo {
		shnum = 2 // drop the .modinfo section header
	}

	modinfoOff := uint64(ehSize)
	shstrtabOff := modinfoOff + uint64(len(modinfoData))
	shoff := shstrtabOff + uint64(len(shstrtab))

	buf := &bytes.Buffer{}
	// ELF header.
	buf.Write([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	le := binary.LittleEndian
	w16 := func(v uint16) { b := make([]byte, 2); le.PutUint16(b, v); buf.Write(b) }
	w32 := func(v uint32) { b := make([]byte, 4); le.PutUint32(b, v); buf.Write(b) }
	w64 := func(v uint64) { b := make([]byte, 8); le.PutUint64(b, v); buf.Write(b) }
	w16(etype)     // e_type
	w16(62)        // e_machine = EM_X86_64
	w32(1)         // e_version
	w64(0)         // e_entry
	w64(0)         // e_phoff
	w64(shoff)     // e_shoff
	w32(0)         // e_flags
	w16(ehSize)    // e_ehsize
	w16(0)         // e_phentsize
	w16(0)         // e_phnum
	w16(shSize)    // e_shentsize
	w16(shnum)     // e_shnum
	w16(shnum - 1) // e_shstrndx (shstrtab is always the last section)

	buf.Write(modinfoData)
	buf.Write(shstrtab)

	// Section headers. Index 0 is the mandatory null section.
	writeSH := func(name, typ uint32, off, size uint64) {
		w32(name)
		w32(typ)
		w64(0) // sh_flags
		w64(0) // sh_addr
		w64(off)
		w64(size)
		w32(0) // sh_link
		w32(0) // sh_info
		w64(1) // sh_addralign
		w64(0) // sh_entsize
	}
	writeSH(0, 0, 0, 0) // SHT_NULL
	if includeModinfo {
		writeSH(nameModinfo, 1, modinfoOff, uint64(len(modinfoData))) // SHT_PROGBITS
	}
	writeSH(nameShstrtab, 3, shstrtabOff, uint64(len(shstrtab))) // SHT_STRTAB

	return buf.Bytes()
}

func TestParseKoValid(t *testing.T) {
	ko := buildTestKo(t, 1 /*ET_REL*/, map[string]string{
		"name":        "kuaf",
		"vermagic":    "6.18.35 SMP mod_unload",
		"license":     "GPL",
		"description": "planted UAF target",
		"depends":     "",
	}, true)

	p, err := ParseKo(ko)
	if err != nil {
		t.Fatalf("ParseKo: %v", err)
	}
	if p.ModName != "kuaf" {
		t.Errorf("ModName = %q, want kuaf", p.ModName)
	}
	if p.Vermagic != "6.18.35 SMP mod_unload" {
		t.Errorf("Vermagic = %q", p.Vermagic)
	}
	if p.License != "GPL" {
		t.Errorf("License = %q", p.License)
	}
	if p.Machine != "EM_X86_64" {
		t.Errorf("Machine = %q, want EM_X86_64", p.Machine)
	}
	if len(p.SHA256) != 64 {
		t.Errorf("SHA256 not a 32-byte hex digest: %q", p.SHA256)
	}
	if p.IsSigned {
		t.Error("unsigned module reported as signed")
	}
}

func TestParseKoRejectsNonELF(t *testing.T) {
	if _, err := ParseKo([]byte("this is not an elf file at all")); err == nil {
		t.Fatal("expected rejection of non-ELF input")
	}
}

func TestParseKoRejectsEmpty(t *testing.T) {
	if _, err := ParseKo(nil); err == nil {
		t.Fatal("expected rejection of empty input")
	}
}

func TestParseKoRejectsNonRelocatable(t *testing.T) {
	// A valid ELF, but ET_EXEC (2), not a module.
	exe := buildTestKo(t, 2, map[string]string{"name": "x", "vermagic": "v"}, true)
	if _, err := ParseKo(exe); err == nil {
		t.Fatal("expected rejection of a non-relocatable ELF")
	}
}

func TestParseKoRejectsNoModinfo(t *testing.T) {
	// A relocatable ELF with no .modinfo section is not a kernel module.
	rel := buildTestKo(t, 1, map[string]string{}, false)
	if _, err := ParseKo(rel); err == nil {
		t.Fatal("expected rejection of an ELF lacking .modinfo")
	}
}

func TestParseKoDetectsSignature(t *testing.T) {
	ko := buildTestKo(t, 1, map[string]string{"name": "signed", "vermagic": "v"}, true)
	ko = append(ko, sigTrailer...)
	p, err := ParseKo(ko)
	if err != nil {
		t.Fatalf("ParseKo: %v", err)
	}
	if !p.IsSigned {
		t.Error("signed module (trailer present) reported as unsigned")
	}
}

func TestSecretKeyValidAndUnique(t *testing.T) {
	// A name with characters illegal in a Secret key must be sanitized, and the
	// id-derived suffix keeps two same-named modules from colliding.
	k1 := secretKey("my/evil name", "whatever.ko", "abcdef0123456789")
	k2 := secretKey("my/evil name", "whatever.ko", "fedcba9876543210")
	if k1 == k2 {
		t.Fatalf("distinct ids produced the same key: %q", k1)
	}
	for _, k := range []string{k1, k2} {
		if strings.ContainsAny(k, "/ ") {
			t.Errorf("key %q contains characters illegal in a Secret key", k)
		}
		if !strings.HasSuffix(k, ".ko") {
			t.Errorf("key %q not .ko-suffixed", k)
		}
	}
}

func TestDisplayNameFallsBackToFilename(t *testing.T) {
	if got := displayName("", "foo.ko"); got != "foo" {
		t.Errorf("displayName fallback = %q, want foo", got)
	}
	if got := displayName("bar", "foo.ko"); got != "bar" {
		t.Errorf("displayName = %q, want bar", got)
	}
}
