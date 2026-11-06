// patch.go writes an edition's release: the xdelta3 patch from the original
// disc image to the edition's disc, a cue sheet for the patched image and a
// README.txt with both images' size and SHA-256. It then applies the patch to
// a temporary file and checks that the result is the edition's disc.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// xdeltaEncodeOptions encode the patch without secondary compression, which
// Debian's xdelta3 turns on by default and some VCDIFF patchers cannot
// decode, at the highest compression level.
var xdeltaEncodeOptions = []string{"-e", "-f", "-9", "-S", "none"}

// releaseReadme is the release's README.txt: the edition's title, the
// release name, the original image's size and SHA-256, then the patched
// image's.
const releaseReadme = `Final Fantasy Tactics: %[1]s

%[2]s.xdelta is an xdelta3 (VCDIFF) patch from the disc image of
Final Fantasy Tactics, USA (SCUS-94221), to the %[1]s, built from
the source at https://github.com/LuizORAS/fft-decomp.

It applies to the disc as one raw BIN of 2352-byte sectors (a single
MODE2/2352 track):
  size    %[3]d bytes
  sha256  %[4]s

It gives %[2]s.bin:
  size    %[5]d bytes
  sha256  %[6]s

Apply it with a VCDIFF patcher such as Delta Patcher, Rom Patcher JS or
xdelta3:

  xdelta3 -d -s original.bin %[2]s.xdelta %[2]s.bin

The patcher checks what it writes, so another image fails with a checksum
error.

Keep %[2]s.cue next to %[2]s.bin and open the
cue sheet in the emulator.
`

// patchCommand is make patch: the release of the edition --edition names,
// made from the disc make patch has just built.
func (p project) patchCommand(args []string) error {
	const usage = "usage: tools patch --edition=E"
	flags := flag.NewFlagSet("patch", flag.ContinueOnError)
	edition := flags.String("edition", "", "edition of target/editions.yaml whose disc the patch gives")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New(usage)
	}
	if *edition == "" {
		return errors.New("make patch makes an edition's patch: give EDITION (make patch EDITION=dc)")
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		return err
	}
	if p, err = p.withEdition(config, *edition); err != nil {
		return err
	}
	return p.writeRelease(config.Editions.edition(p.edition).Title)
}

// writeRelease writes build/<edition>/release: <target>-<edition>.xdelta, the
// patch from the original image to the edition's disc, <target>-<edition>.cue
// and README.txt, once the patch applied to the original gives that disc.
func (p project) writeRelease(title string) error {
	source := filepath.Join(p.root, targetInput)
	sourceSHA, err := sha256File(source)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("make patch needs %s, the disc image the patch applies to", targetInput)
	}
	if err != nil {
		return err
	}
	if sourceSHA != targetInputSHA256 {
		return fmt.Errorf("%s has sha256 %s, not the original disc's %s", targetInput, sourceSHA, targetInputSHA256)
	}
	disc := p.outputDir("disc", targetOutput+".bin")
	discInfo, err := os.Stat(disc)
	if err != nil {
		return fmt.Errorf("no disc of edition %s: run make build EDITION=%s (%w)", p.edition, p.edition, err)
	}
	discSHA, err := sha256File(disc)
	if err != nil {
		return err
	}
	cue, err := os.ReadFile(p.outputDir("disc", targetOutput+".cue"))
	if err != nil {
		return err
	}
	name := targetID + "-" + p.edition
	releaseCue, err := renameCueFile(cue, targetOutput+".bin", name+".bin")
	if err != nil {
		return err
	}

	release := p.outputDir("release")
	if err := os.RemoveAll(release); err != nil {
		return err
	}
	if err := os.MkdirAll(release, 0o755); err != nil {
		return err
	}
	patch := filepath.Join(release, name+".xdelta")
	if err := xdeltaEncode(p.root, source, disc, patch, name+".bin//"+targetInput+"/"); err != nil {
		return err
	}
	applied := filepath.Join(release, name+".bin")
	var appliedSHA string
	err = xdeltaApply(p.root, source, patch, applied)
	if err == nil {
		appliedSHA, err = sha256File(applied)
	}
	if removeErr := os.Remove(applied); err == nil && removeErr != nil {
		err = removeErr
	}
	if err != nil {
		return err
	}
	if appliedSHA != discSHA {
		return fmt.Errorf("the patch applied to %s gives sha256 %s, not the edition's disc's %s", targetInput, appliedSHA, discSHA)
	}
	if err := os.WriteFile(filepath.Join(release, name+".cue"), releaseCue, 0o644); err != nil {
		return err
	}
	readme := fmt.Sprintf(releaseReadme, title, name, targetInputBytes, sourceSHA, discInfo.Size(), discSHA)
	if err := os.WriteFile(filepath.Join(release, "README.txt"), []byte(readme), 0o644); err != nil {
		return err
	}
	patchInfo, err := os.Stat(patch)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(p.root, release)
	if err != nil {
		return err
	}
	fmt.Printf("patch: %s.xdelta, %d bytes, from %s (sha256 %s) to the %s disc (sha256 %s): applied to a temporary copy, it gives that disc\n",
		name, patchInfo.Size(), targetInput, sourceSHA[:16], title, discSHA[:16])
	fmt.Printf("release: %s (%s.xdelta, %s.cue, README.txt)\n", filepath.ToSlash(relative), name, name)
	return nil
}

// renameCueFile points the cue sheet's one FILE line, which names from, at to.
func renameCueFile(cue []byte, from, to string) ([]byte, error) {
	old := []byte(fmt.Sprintf("FILE %q", from))
	if count := bytes.Count(cue, old); count != 1 {
		return nil, fmt.Errorf("the cue sheet names %s %d times, not once", from, count)
	}
	return bytes.Replace(cue, old, []byte(fmt.Sprintf("FILE %q", to)), 1), nil
}

// xdeltaEncode writes the patch from source to target, with header (the
// target's name, then the source's: "target//source/") as its application
// header, which xdelta3 takes as the output's name when it is given none.
func xdeltaEncode(dir, source, target, patch, header string) error {
	arguments := append([]string{"xdelta3"}, xdeltaEncodeOptions...)
	return runInDir(dir, append(arguments, "-A="+header, "-s", source, target, patch)...)
}

// xdeltaApply writes output from source and patch; xdelta3 fails when what it
// writes does not match the checksums the patch carries.
func xdeltaApply(dir, source, patch, output string) error {
	return runInDir(dir, "xdelta3", "-d", "-f", "-s", source, patch, output)
}
