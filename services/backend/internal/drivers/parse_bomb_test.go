package drivers

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// buildKoWithModinfoSection is buildTestKo's hostile sibling: it lets a test
// control the .modinfo section header's flags, type and declared size, and pass
// raw section bytes. That is the whole attack surface readModinfo guards, and
// none of it is reachable through buildTestKo (which always writes a plain
// SHT_PROGBITS section with no flags).
//
// declaredSize is written to sh_size. For an uncompressed section the stdlib
// takes that as the section's size directly; for SHF_COMPRESSED it is the
// on-disk size and the real size comes from the compression header instead.
func buildKoWithModinfoSection(t *testing.T, secFlags uint64, secType uint32, section []byte, declaredSize uint64) []byte {
	t.Helper()

	shstrtab := append([]byte{0}, []byte(".modinfo\x00.shstrtab\x00")...)
	const nameModinfo, nameShstrtab = uint32(1), uint32(10)
	const ehSize, shSize = 64, 64

	modinfoOff := uint64(ehSize)
	shstrtabOff := modinfoOff + uint64(len(section))
	shoff := shstrtabOff + uint64(len(shstrtab))

	buf := &bytes.Buffer{}
	le := binary.LittleEndian
	w16 := func(v uint16) { b := make([]byte, 2); le.PutUint16(b, v); buf.Write(b) }
	w32 := func(v uint32) { b := make([]byte, 4); le.PutUint32(b, v); buf.Write(b) }
	w64 := func(v uint64) { b := make([]byte, 8); le.PutUint64(b, v); buf.Write(b) }

	buf.Write([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	w16(1)      // e_type = ET_REL
	w16(62)     // e_machine = EM_X86_64
	w32(1)      // e_version
	w64(0)      // e_entry
	w64(0)      // e_phoff
	w64(shoff)  // e_shoff
	w32(0)      // e_flags
	w16(ehSize) // e_ehsize
	w16(0)      // e_phentsize
	w16(0)      // e_phnum
	w16(shSize) // e_shentsize
	w16(3)      // e_shnum
	w16(2)      // e_shstrndx

	buf.Write(section)
	buf.Write(shstrtab)

	writeSH := func(name, typ uint32, flags, off, size uint64) {
		w32(name)
		w32(typ)
		w64(flags)
		w64(0) // sh_addr
		w64(off)
		w64(size)
		w32(0) // sh_link
		w32(0) // sh_info
		w64(1) // sh_addralign
		w64(0) // sh_entsize
	}
	writeSH(0, 0, 0, 0, 0) // SHT_NULL
	writeSH(nameModinfo, secType, secFlags, modinfoOff, declaredSize)
	writeSH(nameShstrtab, 3, 0, shstrtabOff, uint64(len(shstrtab)))

	return buf.Bytes()
}

// zlibOfZeros returns n zero bytes zlib-compressed. Zeroes pack roughly 1000:1,
// which is what makes the bomb cheap to send and expensive to receive.
func zlibOfZeros(t *testing.T, n int) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	if _, err := zw.Write(make([]byte, n)); err != nil {
		t.Fatalf("compressing: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zlib writer: %v", err)
	}
	return out.Bytes()
}

// A .modinfo flagged SHF_COMPRESSED makes the stdlib hand Section.Data() a live
// decompressor sized by the uploader's own compression header, so a few KB of
// upload expands to whatever it declares. MaxKoBytes bounds the upload, not the
// expansion, so before readModinfo this parsed happily and allocated the full
// declared size - on a 256Mi backend pod, an OOM kill of the shared process.
//
// The test declares 4 MiB rather than the gigabytes a real attack would use: the
// point is that the guard refuses BEFORE decompressing, and 4 MiB keeps the test
// fast. Note the pre-fix behaviour here is a clean success, not an error, so
// this fails loudly if the guard is ever removed.
func TestParseKoRejectsCompressedModinfo(t *testing.T) {
	const declared = 4 * 1024 * 1024

	var chdr bytes.Buffer
	le := binary.LittleEndian
	b4, b8 := make([]byte, 4), make([]byte, 8)
	le.PutUint32(b4, 1) // ch_type = ELFCOMPRESS_ZLIB
	chdr.Write(b4)
	le.PutUint32(b4, 0) // ch_reserved
	chdr.Write(b4)
	le.PutUint64(b8, declared) // ch_size - attacker-declared expansion
	chdr.Write(b8)
	le.PutUint64(b8, 1) // ch_addralign
	chdr.Write(b8)

	section := append(chdr.Bytes(), zlibOfZeros(t, declared)...)
	// SHF_COMPRESSED (0x800), and deliberately NOT SHF_ALLOC: the stdlib rejects
	// the two together, so a real attacker clears ALLOC - and nothing else in the
	// parser looked at flags.
	ko := buildKoWithModinfoSection(t, 0x800, 1 /*SHT_PROGBITS*/, section, uint64(len(section)))

	if len(ko) > 512*1024 {
		t.Fatalf("bomb should be small on the wire, got %d bytes", len(ko))
	}

	_, err := ParseKo(ko)
	if err == nil {
		t.Fatal("compressed .modinfo was accepted: the decompression-bomb guard is gone")
	}
	if !errors.Is(err, ErrNotKo) {
		t.Fatalf("error should map to ErrNotKo (400 at the edge), got %v", err)
	}
	if !strings.Contains(err.Error(), "compressed") {
		t.Fatalf("expected the compression guard to fire, got %v", err)
	}
}

// A section can also just declare a huge plain size. Data() would allocate it
// before reading a byte, so the size ceiling has to apply to uncompressed
// sections too, not only to the compressed path.
func TestParseKoRejectsOversizedModinfo(t *testing.T) {
	section := []byte("name=oversized\x00")
	ko := buildKoWithModinfoSection(t, 0, 1 /*SHT_PROGBITS*/, section, maxModinfoBytes+1)

	_, err := ParseKo(ko)
	if err == nil {
		t.Fatal("oversized .modinfo was accepted")
	}
	if !errors.Is(err, ErrNotKo) {
		t.Fatalf("error should map to ErrNotKo, got %v", err)
	}
	if !strings.Contains(err.Error(), "over the") {
		t.Fatalf("expected the size guard to fire, got %v", err)
	}
}

// SHT_NOBITS occupies no file bytes - sh_size is purely declarative - so reading
// it would both invent data and allocate the declared size.
func TestParseKoRejectsNobitsModinfo(t *testing.T) {
	ko := buildKoWithModinfoSection(t, 0, 8 /*SHT_NOBITS*/, nil, 64*1024)

	_, err := ParseKo(ko)
	if err == nil {
		t.Fatal("SHT_NOBITS .modinfo was accepted")
	}
	if !errors.Is(err, ErrNotKo) {
		t.Fatalf("error should map to ErrNotKo, got %v", err)
	}
	if !strings.Contains(err.Error(), "SHT_NOBITS") {
		t.Fatalf("expected the NOBITS guard to fire, got %v", err)
	}
}

// The guards must not cost us real modules: an ordinary uncompressed .modinfo
// built by the same hostile builder still parses.
func TestParseKoAcceptsPlainModinfoViaSectionBuilder(t *testing.T) {
	section := []byte("name=demo\x00vermagic=6.9.0\x00license=GPL\x00")
	ko := buildKoWithModinfoSection(t, 0, 1 /*SHT_PROGBITS*/, section, uint64(len(section)))

	got, err := ParseKo(ko)
	if err != nil {
		t.Fatalf("plain .modinfo should parse, got %v", err)
	}
	if got.ModName != "demo" || got.Vermagic != "6.9.0" || got.License != "GPL" {
		t.Fatalf("metadata not extracted: %+v", got)
	}
}
