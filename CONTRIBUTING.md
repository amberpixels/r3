# Contributing to R3

First of all - thank you for caring enough to open this file. That genuinely
means a lot. 💛

R3 is currently built and maintained as a small, focused solo project, and here's
where things stand right now.

## Pull requests are not open (for now)

**R3 is not accepting code contributions / pull requests at this stage.**

Rather than let PRs sit unreviewed, I'd rather be upfront: please don't invest
your time in a pull request here yet. If that changes, this file will change with
it.

(On how the code itself is written, see the [AI disclosure](readme.md#ai-disclosure).)

## What *is* very welcome

Lots! I'd love to hear from you:

- 🐛 **Bug reports** - if something is broken or surprising, please
  [open an issue](https://github.com/amberpixels/r3/issues). Reproductions are gold.
- 💡 **Ideas & feature suggestions** - tell me what you'd want R3 to do.
- ❓ **Questions** - about the design, the layering, how to use it, or whether it
  fits your use case. No question is too small.
- 🗣️ **Feedback & experience reports** - if you tried R3 (or just read the code),
  what felt good, what felt off? That kind of signal shapes the project.

Issues and discussions are open and I read them. Kindness and curiosity are
always answered in kind.

## Using R3 in your own work

R3 is MIT-licensed, so you're free to **fork it, vendor it, and adapt it** for
your own projects however you like. If you fork it locally and want to build and
test:

R3 uses [`just`](https://github.com/casey/just) as a command runner.

```bash
git clone https://github.com/amberpixels/r3.git
cd r3
go mod download
```

| Command | What it does |
|---------|--------------|
| `just` / `just tidy` | `go fmt`, `go vet`, `go mod tidy` |
| `just lint` | Run golangci-lint (installs it if missing) |
| `just test` | Run all tests (**requires Docker** for testcontainers) |
| `just test-short` | Run short tests only (no Docker) |
| `just example` | Run the pet store example (starts Postgres in Docker) |

The architecture is documented in the [README](readme.md#architecture) and in the
`doc.go` of each package, in case you want to find your way around.

## Security

If you find a security vulnerability, please report it privately - see
[SECURITY.md](SECURITY.md). Don't open a public issue for it.

---

Thanks again for being here. Software gets better when people are kind to each
other, so - be kind. ✨
