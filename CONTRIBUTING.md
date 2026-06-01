# Contributing to Makework

Thanks for considering a contribution. Before opening a PR, please read:

1. [`AGENTS.md`](./AGENTS.md) — repo-local conventions, dev loop, and
   definition of done.
2. [Engineering principles](https://github.com/Jylhis/virt-corp/blob/main/docs/ENGINEERING_PRINCIPLES.md)
   — the 15 principles that govern every Jylhis project.
3. [Way of working](https://github.com/Jylhis/virt-corp/blob/main/docs/WAY_OF_WORKING.md)
   — the day-to-day norms (PR size, reviews, releases).

## Dev loop in one paragraph

```sh
devenv shell           # or: just dev
just check             # build + lint + tests + fmt
just test              # fast inner loop
```

`mw` is a single Go module (`github.com/jylhis/makework`); the binary lives in
[`cmd/mw`](./cmd/mw) and all logic is under [`internal/`](./internal). See
[`CLAUDE.md`](./CLAUDE.md) for the package map.

## Filing issues

Bug reports and feature requests go to
[GitHub Issues](https://github.com/Jylhis/Makework/issues). Include the `mw`
version (`mw --version`), Go version, and the command you ran.
