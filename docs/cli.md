# The svcview CLI

## Install

The installer downloads the latest release for your platform, verifies its checksum, and puts the
binary where you can start using it. It needs `sh`, `curl`, `jq`, `tar`, and `gzip`.

```shell
sh <(curl -fsSL https://raw.githubusercontent.com/hpcsc/svcview/main/scripts/install.sh)
```

It asks for the release channel and the install directory. To skip the questions on the command line:

```shell
sh <(curl ...) --channel release --dir ~/.local/bin
```

The channel is `release` (the latest stable release) or `prerelease` (the latest build of `main`).
Without `--dir`, the binary lands in `~/.local/bin` (or `$INSTALL_DIR`). When more than one version is
available in the channel, the installer lists them for you to pick.

`GITHUB_TOKEN` or `GH_TOKEN` gives access to a private repository.

`svcview update` also installs from a release, and it replaces this very binary, so the
installer and the update command do the same kind of job; use whichever you find convenient on a fresh
machine.

## Build

```shell
task build             # build ./bin/svcview
task cli -- --help     # run the CLI from the source, with the arguments after --
```

## Version and update

```shell
svcview version                # the tag of a release or a prerelease, or the commit of any other build
svcview update                 # install the latest release
svcview update --prerelease    # install the latest prerelease, a build of main
svcview update --check         # only tell you whether this build is the latest
```

`internal/cli/version` reports the version. A release build gets its tag from goreleaser, through
`-ldflags -X .../internal/cli/version.releaseTag`. Any other build reports the short commit sha that Go
keeps in the build information, with `-dirty` after it when the working tree had changes.

`svcview update` downloads the archive for your platform from a GitHub release, checks it
against the `checksums.txt` of that release, and then replaces the binary. It names each step on
stderr, and in a terminal it shows how much of the download has arrived. `GITHUB_TOKEN` or `GH_TOKEN`
gives access to a private repository.

Releases and prereleases are two channels. Each command installs the latest build of its channel when
this build is a different one, so `svcview update` on a prerelease goes back to the latest
release. A build from a commit is not a release or a prerelease, so `svcview update` does not
replace it unless you add `--force`.

## Release

GoReleaser builds the releases from `.goreleaser.yaml`. The CI workflow, `.github/workflows/ci.yml`, publishes
them after its jobs `test` and `e2e` pass:

- A push of a tag `v<major>.<minor>.<patch>` publishes a release.
- Each push to `main` publishes a prerelease. The job `prerelease` tags the commit with the next patch after the
  latest release, the run number and the commit, for example `v0.2.1-42.g4829f92`. It keeps only the 5 newest
  prereleases. A run whose commit is no longer the newest commit of `main` publishes nothing, so an older build
  never replaces a newer prerelease.
- A re-run of a `prerelease` job that pushed its tag fails at the tag. The next push to `main` publishes a new
  prerelease.
- When three pushes to `main` come close together, the newest commit can get no prerelease: the `prerelease` job
  of an older commit can cancel the pending `prerelease` job of the newest commit. The next push to `main`
  publishes a new prerelease.

```shell
git tag vX.X.X
git push origin vX.X.X
```

`task release:local` makes a snapshot build in `dist/`, and publishes nothing.

## E2E Test

The end-to-end tests run the built binary in a terminal, send keys to it, and read the screen. They are
in `e2e/`, and [e2e-tests.md](e2e-tests.md) tells how they work.

```shell
task test:e2e          # in Docker
task test:e2e:local    # on this machine, with Node
```
