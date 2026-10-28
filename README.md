# Final Fantasy Tactics decompilation

> Status: 100% decompiled and byte-exact ✅

## About this fork

A fork of [adamrt/fft_decomp](https://github.com/adamrt/fft_decomp) that makes
the source readable, modifiable and portable without reverse-engineering it
again. The rule is the same as upstream: every commit still builds the
original disc byte for byte, checked with `make check`. `master` mirrors
upstream.

- **Build 1** (tag and branch `build-1`): names for what was
  unnamed, real types for data, and ordinary C in place of leftover `goto`s
  and register pins.
- **Build 2** (tag and branch `build-2`): a map of the codebase.
  [`CODEBASE.md`](CODEBASE.md) explains the architecture,
  [`docs/`](docs/README.md) explains each game mechanic and where to change
  it, and `make map` generates an Obsidian vault with a page for every
  function, type and global. Every function has a summary.

A matching decompilation of the North American PlayStation release of *Final
Fantasy Tactics* (`SCUS-94221` [redump](http://redump.org/disc/55/)). Every game
function is C that compiles to the original bytes, and the rebuilt disc is a
byte-for-byte match.

Game files and proprietary Sony tools are not included.

## Next Steps

The game is 100% completely decompiled, but there are plenty of semantic changes
and cleanup to be done. Variable and function names, correcting types, improving
enum usage/naming, etc.

Each time a change is made `make check` will ensure the changes still match
byte-for-byte.

## Getting started

Requirements
- Docker
- `SCUS-94221` BIN at `./scus-94221.bin` (optional)

Then run:

```sh
# Prepare environment
make bootstrap

# Then use one of the following (make help for more):
make check     # verify all code against checksums (without BIN)
make build     # link every module and a byte-matching disc (the disc needs the BIN)
```

`make build` writes `build/disc/output-scus-94221.bin` and `.cue`.

## Thanks

This project is an ode to the [FFHacktics](https://ffhacktics.com/wiki/)
community. ❤️

And thanks to Adam ([@adamrt](https://github.com/adamrt)) for the decompilation
this fork builds on, and for the go-ahead to fork it. 🙏⚔️✨
