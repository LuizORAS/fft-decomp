package main

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRenameCueFileNamesThePatchedImage(t *testing.T) {
	cue := []byte("FILE \"output-scus-94221.bin\" BINARY\n  TRACK 01 MODE2/2352\n    INDEX 01 00:00:00\n")
	got, err := renameCueFile(cue, "output-scus-94221.bin", "scus-94221-dc.bin")
	if err != nil {
		t.Fatal(err)
	}
	want := "FILE \"scus-94221-dc.bin\" BINARY\n  TRACK 01 MODE2/2352\n    INDEX 01 00:00:00\n"
	if string(got) != want {
		t.Fatalf("renameCueFile = %q, want %q", got, want)
	}
	if _, err := renameCueFile(cue, "other.bin", "scus-94221-dc.bin"); err == nil {
		t.Fatal("renameCueFile accepted a cue sheet that does not name the image")
	}
}

// The patch has no secondary compression and names both images in its
// header; encoding again gives the same bytes; applied to its source it gives
// the target, and applied to another image it fails.
func TestXdeltaPatchRoundTrip(t *testing.T) {
	dir := t.TempDir()
	source := make([]byte, 1<<20)
	rand.New(rand.NewSource(1)).Read(source)
	target := append(slices.Clone(source[:300000]), []byte("a fix that grows the code")...)
	target = append(target, source[300000:]...)
	target[700000] ^= 0xff
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	write("a.bin", source)
	write("b.bin", target)

	if err := xdeltaEncode(dir, "a.bin", "b.bin", "b.xdelta", "b.bin//a.bin/"); err != nil {
		t.Fatal(err)
	}
	patch := read("b.xdelta")
	// VCDIFF: the magic d6 c3 c4 and version 0, then the header indicator:
	// no secondary compressor (0x01) and an application header (0x04).
	if len(patch) < 5 || !bytes.Equal(patch[:4], []byte{0xd6, 0xc3, 0xc4, 0x00}) {
		t.Fatalf("patch starts % x, not a VCDIFF header", patch[:min(len(patch), 5)])
	}
	if patch[4]&0x01 != 0 || patch[4]&0x04 == 0 {
		t.Fatalf("header indicator 0x%02x: want no secondary compressor and an application header", patch[4])
	}
	if !bytes.Contains(patch, []byte("b.bin//a.bin/")) {
		t.Fatal("the patch's application header does not name both images")
	}

	if err := xdeltaEncode(dir, "a.bin", "b.bin", "again.xdelta", "b.bin//a.bin/"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read("again.xdelta"), patch) {
		t.Fatal("encoding the same images again gave other bytes")
	}

	if err := xdeltaApply(dir, "a.bin", "b.xdelta", "out.bin"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read("out.bin"), target) {
		t.Fatal("the patch applied to its source does not give the target")
	}

	wrong := slices.Clone(source)
	wrong[100] ^= 1
	write("wrong.bin", wrong)
	if err := xdeltaApply(dir, "wrong.bin", "b.xdelta", "wrong-out.bin"); err == nil {
		t.Fatal("the patch applied to another image did not fail")
	}
}
