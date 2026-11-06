# Final Fantasy Tactics decompilation

[![check](https://github.com/LuizORAS/fft-decomp/actions/workflows/check.yml/badge.svg)](https://github.com/LuizORAS/fft-decomp/actions/workflows/check.yml)

> Status: 100% decompiled; the rebuilt disc matches the original byte for byte ✅

A matching decompilation of the North American PlayStation release of *Final
Fantasy Tactics* (`SCUS-94221`, [redump](http://redump.org/disc/55/)). Every
game function is C that compiles to the original bytes, except the routines
the original programmers wrote in assembly, which are `.s` files that
assemble to them. The Sony libraries the game links are reconstructed the
same way. `make build` links every module whole and writes a disc that is
identical to the original.

Game files and proprietary Sony tools are not included: the build reads the
game's data from your own copy of the disc.

## Where it is going

The build without options will always produce the original disc. On top of
it, the same source is meant to build:

- **Developer's Cut:** the PlayStation game with its bugs fixed, released as
  a patch for your own disc;
- **Enhanced Edition:** native ports of the same game for other platforms;
- **mods**, kept in this repository.

## Milestones

- **Build 1** (tag `build-1`): names for what was unnamed, real types for
  data, and ordinary C in place of leftover `goto`s and register pins.
- **Build 2** (tag `build-2`): a map of the codebase.
  [`CODEBASE.md`](CODEBASE.md) explains the architecture,
  [`docs/`](docs/README.md) explains each game mechanic and where to change
  it, and `make map` generates an Obsidian vault with a page for every
  function, type and global. Every function has a summary.
- **Build 2.5** (tag `build-2.5`): the shiftable build. `make build` links
  each module whole, and every word that holds an address or a disc sector
  follows what it names, while the disc still comes out byte for byte;
  `make build SHIFT=16` and `make smoke` prove it on a disc where everything
  moves.
- **Build 3.0** (tag `build-3.0`): a repository of its own, with the whole
  history, the MIT license, credits to the original decompilation and a
  protected `main` that takes changes by pull request with CI green.
- **Build 3.1a** (tag `build-3.1a`): the Developer's Cut can be built. The
  same source makes a second disc: a change lives behind an option, a guard
  proves that nothing else changed, the code after a function that grows
  moves with everything that points to it, `make scenario` shows a change on
  both discs and `make patch` writes the release. One fix went through the
  whole path: unloading the playing music now clears its state.

## Getting started

Requirements:
- Docker (the engine alone is enough);
- the `SCUS-94221` BIN at `./scus-94221.bin` (optional: `make check` and
  `make build` check every function against its hash without it).

Then run:

```sh
# Prepare environment
make bootstrap

# Then use one of the following (make help for more):
make check     # verify all code against checksums (without BIN)
make build     # link every module and a byte-matching disc (the disc needs the BIN)
```

`make build` writes `build/disc/output-scus-94221.bin` and `.cue`.

## The Developer's Cut

The same source builds the Developer's Cut, which changes retail behavior
(bug fixes first) behind the options of
[`target/editions.yaml`](target/editions.yaml), while the build without an
edition stays the original disc:

```sh
make build EDITION=dc   # its disc in build/dc/disc (needs the BIN)
make patch EDITION=dc   # its xdelta3 patch from your BIN, in build/dc/release
```

`make patch` writes `scus-94221-dc.xdelta`, a cue sheet for the patched image
and a `README.txt` with both discs' size and SHA-256 and how to apply the
patch (Delta Patcher, Rom Patcher JS or `xdelta3`). The changes it holds are
the [`QUIRKS.md`](QUIRKS.md) entries marked "Fixed under"; `make map`'s
Build 3 backlog page counts them. [`AGENTS.md`](AGENTS.md) ("Changes under
an option") has the rules for adding one.

## Documentation

- [`CODEBASE.md`](CODEBASE.md): how the game fits together and where to find
  things.
- [`docs/`](docs/README.md): one page per game mechanic, and guides for
  common changes.
- [`AGENTS.md`](AGENTS.md): the working rules (layout, names, types,
  matching) and every `make` command.
- [`QUIRKS.md`](QUIRKS.md): retail bugs and code that looks wrong on purpose.

## Credits

- **Original source:** [adamrt/fft_decomp](https://github.com/adamrt/fft_decomp)
  by Adam Patterson ([@adamrt](https://github.com/adamrt)), the decompilation
  this project started from. Its history is kept here, and he gave the
  go-ahead to build on it. 🙏⚔️✨
- **[FFHacktics](https://ffhacktics.com/wiki/):** this project is an ode to
  its community. ❤️

## License

The source code, tools and documentation are released under the
[MIT License](LICENSE).
