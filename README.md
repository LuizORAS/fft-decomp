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

## Documentation

- [`CODEBASE.md`](CODEBASE.md): how the game fits together and where to find
  things.
- [`docs/`](docs/README.md): one page per game mechanic, and guides for
  common changes.
- [`AGENTS.md`](AGENTS.md): the working rules (layout, names, types,
  matching) and every `make` command.
- [`QUIRKS.md`](QUIRKS.md): retail bugs and code that looks wrong on purpose.

## Thanks

This project is an ode to the [FFHacktics](https://ffhacktics.com/wiki/)
community. ❤️

And thanks to Adam ([@adamrt](https://github.com/adamrt)) for
[the decompilation](https://github.com/adamrt/fft_decomp) this project
started from, and for the go-ahead to build on it. 🙏⚔️✨
