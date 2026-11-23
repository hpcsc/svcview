## Contents
1. Task 1: Read the ACL of an open file on macOS
2. Task 2: Refuse a config file that has an ACL
3. Task 3: Read the status command of a group
4. Task 4: Describe the refusal of a config file with an ACL in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-003 "See the status that each service reports" (`launchd-tui/US-003`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "Settings of a Group", "Template Values" and "Config Files".

This change lets a group of the config file hold a status command, and it makes sure that only the user of svcview
can write the file that holds it. The setting `status` of a group has `command`, `every` and `timeout`. `command` is
the program and its arguments, and each of them is a Go template that can use the template values `{{.Name}}`,
`{{.Env.X}}`, `{{.StdoutPath}}`, `{{.StderrPath}}` and `{{.PlistPath}}`. `every` is `2s` by default, and `timeout`
is `1s`. An invalid template, or any other error in `status`, stops svcview at start, and the error names the file
and the group. svcview runs no status command.

The config file now holds commands that svcview runs as its user. svcview already refuses `config.yaml` when its
group or other users can write it, or when another user owns it. On macOS, an ACL can let another user write a file
whose mode is `0644`. So svcview also refuses `config.yaml` when the file has an ACL with one or more entries, and the
error names the file.

The story criterion that this change satisfies in part:

- Each argument can use the template values `{{.Name}}`, `{{.Env.X}}`, `{{.StdoutPath}}`, `{{.StderrPath}}`, and
  `{{.PlistPath}}`, from the plist of the service. A value that the plist does not have shows as the status of that
  service, for example `the plist has no WORKER_STATUS_FILE`. An invalid template stops svcview at start, and the
  error names the file and the group.

Of this criterion, this change gives the five template values that an argument can use, and the stop at start for
an invalid template.

It also satisfies, for a macOS ACL, the criterion of US-002 "svcview refuses to start when another user can write
the config file. The error names the file." (`tasks/put-the-services-in-groups-with-a-config-file/decisions.md`,
entry 2).

**The rule for an ACL.**

svcview refuses `config.yaml` when its ACL has one or more entries, whatever each entry allows or denies. It does
not refuse only the entries that allow a write to a user other than the owner, for these reasons:

- An ACL entry names a user or a group by a GUID, not by a user ID. The owner of the file has no GUID in what the
  kernel gives back (`ATTR_CMN_UUID` is the null GUID on APFS), and a GUID becomes a user ID only through the
  membership functions of libc, which a build with `CGO_ENABLED=0` cannot call. So svcview cannot tell an entry of
  the owner from an entry of another user.
- Without the owner, a rule on the rights of each entry refuses the same files as this rule, except a file whose
  entries only deny, or only allow a read. Such a rule needs a list of each right that lets another user change the
  file: write, append, write the ACL, take the file, and the generic rights. A right missing from that list lets a
  writable file through, and only a Mac can test the list.
- A `config.yaml` with an ACL is rare, and `chmod -N` removes the ACL in one command. The error says so.
- The check of the owner already refuses when it is not sure: it refuses a file that another user owns also when
  only its owner can write it.

An ACL with no entries, for example after `chmod -a# 0` removed its only entry, gives no rights to anyone, and
svcview reads that file.

**How svcview reads the ACL.**

svcview asks the kernel for the extended security of the open file: `fgetattrlist` with `ATTR_CMN_EXTENDED_SECURITY`
on the file descriptor that `config.Read` already opened. `golang.org/x/sys/unix` has no `Getattrlist` or
`Fgetattrlist` wrapper, but it has `SYS_FGETATTRLIST`, `Attrlist`, `ATTR_BIT_MAP_COUNT`,
`ATTR_CMN_EXTENDED_SECURITY` and `Syscall6`, and on darwin `Syscall6` is a direct kernel trap, not a call to the
variadic `syscall()` of libc. svcview does not parse `ls -le`, for these reasons:

- The check reads the same open file as the check of the mode and the owner. A parse of `ls -le` checks a path, and
  for a symbolic link `ls` shows the ACL of the link, not of the file that svcview reads.
- `ls` on the `PATH` of the user can be another program. On the Mac of this repo, `ls` and `chmod` in a shell are
  the GNU programs, which have no `-e` and no `+a`. svcview would have to run `/bin/ls`, and parse a text that is not
  an API.
- A run of `ls` at each start adds a program that can fail or hang before the screen opens.

The planner ran a probe of this call on this Mac (Darwin 27.2, arm64, and amd64 under Rosetta). The findings are in
Codebase Context.

## Boundaries

**Out of scope:**
- No systemd, and no other service manager.
- No services of the system domain, and no services in `/Library/LaunchDaemons`. They need root.
- No install, edit, or delete of a plist. The program that owns the service does that.
- No run history. launchd keeps none, and svcview keeps none.
- No releases for Linux or Windows.
- No `launchctl disable` on a stop. A stopped service starts again at the next login.
- No work in the worker repo. The worker writes its status file, and its install script adds `WORKER_STATUS_FILE`
  to the plist and writes `conf.d/worker.yaml`. That work is outside svcview.
- svcview reads the config file only at start. A change to the config file while svcview runs takes effect at the
  next start.
- svcview reads only `config.yaml`. It reads no `conf.d/` folder.
- svcview checks the ACL of `config.yaml`, not of the folders that hold it, as it checks the mode and the owner of
  the file only.
- svcview does not tell an ACL entry of the owner from an entry of another user. Any entry refuses the file.
- On systems other than macOS, svcview reads no ACL. The mode of the file shows a POSIX ACL entry that lets another
  user write it, because that entry sets the group write bit through the ACL mask.
- A group has no `stale_after` in `status`, and no `actions`. Each of them stops svcview at start, as a setting
  that svcview does not know.
- The template values are the five of the proposal. An argument cannot use another property of the plist, for
  example `ProgramArguments`.

**Deferred:**
- svcview runs no status command, and the list shows no status. A group with `status` changes nothing on the screen.
- svcview does not read the environment and the log paths of a plist. It checks only that each argument uses the
  five template values.
- `docs/cli.md` does not describe the `status` settings, the template values, or the errors of `status`.

## Codebase Context

- `internal/config/config.go:27-54`: `config.Read(folder, uid)` opens `config.yaml`, then checks the owner and the
  mode of the open file with `Stat` on the file (`config.go:38-45`), so a symbolic link counts with the file that it
  points to, and a swap of the file between the check and the read has no effect. The tests of the check are at
  `internal/config/config_test.go:212-266`, and they write each `config.yaml` into `t.TempDir()`. `config.Read` has
  one caller, `showScreen` (`internal/cli/root.go:52-66`), and about 45 calls in `config_test.go`, most of them
  through the helpers at `config_test.go:15-29`.
- `go.mod` has `golang.org/x/sys` v0.47.0 as an indirect dependency. Its darwin files have `Setattrlist` but no
  `Getattrlist` (`tasks/learnings.md`, "A check that another user can write a file must read the macOS ACL").
  `unix.Syscall6` on darwin links to `syscall.Syscall6`, which is a direct `SVC`/`SYSCALL` trap
  (`$(go env GOROOT)/src/syscall/asm_darwin_arm64.s`).
- A probe during planning, on this Mac, called `fgetattrlist` through `Syscall6` with `ATTR_CMN_EXTENDED_SECURITY |
  ATTR_CMN_UUID` on an open file, and found:
  - The buffer starts with its length as a `uint32`. The attribute reference of the extended security follows: an
    `int32` offset, relative to the start of the reference, and a `uint32` length.
  - A file with no ACL gives a length of 0. So does a file after `/bin/chmod -N`.
  - At the offset is a `kauth_filesec` in host byte order: the magic `0x012cc16d`, the GUID of the owner and the GUID
    of the group (16 bytes each, both null on APFS), the entry count (`uint32`), the ACL flags (`uint32`), and then
    one entry of 24 bytes for each count: a GUID, the flags (`1` allow, `2` deny), and the rights (for example `4`
    write, `0x20` append, `0x10` delete).
  - `/bin/chmod +a "everyone allow write"` and `+a "user:nobody allow append"` gave a count of 2;
    `+a "everyone deny delete"` gave a count of 1; an ACL whose only entry `chmod -a# 0` removed gave a count of 0.
  - The same program built with `GOARCH=amd64` gave the same bytes under Rosetta.
- On this Mac, `chmod` and `ls` in a shell are the GNU programs of Homebrew (`gchmod`, `gls`), which have no `+a`
  and no `-e`. Tests and docs use `/bin/chmod` and `/bin/ls`.
- The unit tests and the Docker e2e tests run on Linux in CI (`.github/workflows/ci.yml`), so a test that needs a
  macOS ACL skips there. The implementation runs on this Mac, so `mise exec -- task test:unit` and
  `task test:e2e:local` run those tests. `.goreleaser.yaml` builds linux and darwin with `CGO_ENABLED=0`. No file in
  the repo has a build constraint yet; the ACL read is the first code that differs on darwin.
- `internal/config/group.go:34-66`: `readGroup` reads `name` and `match`, and refuses any other setting with an
  error in the form `line N: ... of group "<name>"`. `config.go:98-105` reads a Go duration more than 0 for
  `refresh`. The tests of the group errors are at `config_test.go:348-432`.
- `internal/tui/screen.go:117-120` finds the group of a row with `slices.Index` on `[]config.Group`, so
  `config.Group` must stay comparable. A status command held by value, with a slice of templates in it, would not
  compile there; a pointer keeps the group comparable.
- No code in the repo uses `text/template` yet. The proposal names it for the template values ("Code Layout"). The
  proposal puts the code that runs status commands in `internal/status`, which does not exist yet.
- e2e: `writeConfig(configFolder, content, mode)` (`e2e/testUtils.ts:76-81`) writes `config.yaml`;
  `e2e/tests/config.test.ts:47-58` tests a file that all users can write, and `config.test.ts:16-28` an unknown
  setting. An error that names a path under the macOS `TMPDIR` breaks across lines, so the tests join the lines of
  the screen first (`tasks/learnings.md`). `docs/e2e-tests.md:143-203` describes the config helper and where the
  tests run.
- `docs/cli.md:170-212` lists the errors of the config file, and says that svcview checks the mode and the owner,
  not the ACL.
- Words: "status command" is the program and its arguments; "template value" is a value that an argument can use;
  "every" and "timeout" are the settings, not "interval" or "deadline"; "refresh" stays the read of the services.
  "ACL" is the macOS word, also in `docs/cli.md`.

## Tasks

### Task 1: Read the ACL of an open file on macOS

**Behavior:** `config.ACLEntries` gives the number of entries in the ACL of an open file. On macOS it asks the
kernel for the extended security of the file descriptor. A file with no ACL, or with an ACL with no entries, gives
0. An answer of the kernel that svcview cannot read gives an error. On other systems it gives 0 for each file.

**Acceptance Criteria:**
- [ ] On macOS, a file with no ACL gives 0 entries, and so does a file after `/bin/chmod -N`.
- [ ] On macOS, a file with one ACL entry gives 1, for an entry that allows a right and for an entry that only
      denies; a file with two entries gives 2.
- [ ] On macOS, a file whose ACL lost its only entry through `/bin/chmod -a# 0` gives 0.
- [ ] The count is that of the open file, so a file opened through a symbolic link gives the count of the file that
      the link points to.
- [ ] An answer that does not start with the magic of the extended security, or that is shorter than its entry
      count says, gives an error, not a count.
- [ ] On other systems, each file gives 0.
- [ ] The tests that change an ACL run only on macOS, and use `/bin/chmod`. On other systems they skip.
- [ ] `go build ./...` passes for `GOOS=darwin` with `GOARCH=arm64` and `GOARCH=amd64`, and for `GOOS=linux`.

**Affected Files/Modules:**
- `internal/config/acl_darwin.go` — new: the read of the extended security of an open file on macOS
- `internal/config/acl_other.go` — new: no ACL on other systems
- `internal/config/acl_test.go` — new: the tests, with a real ACL on macOS
- `go.mod` — `golang.org/x/sys` becomes a direct dependency

**Patterns to Follow:**
- `internal/config/config.go:38-45`
- `internal/config/config_test.go:248-266`

**Testable:** Yes — through `config.ACLEntries` on files in `t.TempDir()`, with real ACLs on macOS.

**Certainty:** low — no code in this repo makes a raw system call or reads a structure of the kernel, and it adds
the first file with a build constraint; the probe in Codebase Context gives the layout.

**Blast radius:** high — this read decides whether a file that another user can write through an ACL passes the
check of the config file, which holds commands that svcview runs as its user.

**Verification:** `mise exec -- task test:unit` passes on this Mac, with the macOS tests run and not skipped.
`GOOS=darwin GOARCH=amd64 mise exec -- go build ./...`, `GOOS=darwin GOARCH=arm64 mise exec -- go build ./...` and
`GOOS=linux mise exec -- go vet -tags=unit ./...` pass.

**Depends on:** None

### Task 2: Refuse a config file that has an ACL

**Behavior:** `config.Read` takes, from its caller, the function that counts the ACL entries of the open file.
After the check of the owner and the mode, it refuses `config.yaml` when the file has one or more ACL entries, and
when the count cannot be read. The error names the file, says that the file has an ACL that can let another user
write it, and says that `chmod -N` removes the ACL. svcview gives `config.ACLEntries` to `config.Read` when it opens
the screen.

**Acceptance Criteria:**
- [ ] With a count of 1 or more, `config.Read` gives an error that names the file and `chmod -N`, and no settings,
      also when the owner and the mode pass.
- [ ] When the count cannot be read, `config.Read` gives an error that names the file, and no settings.
- [ ] With a count of 0, `config.Read` gives the settings of the file.
- [ ] With no `config.yaml`, `config.Read` counts no ACL, and the settings are the defaults.
- [ ] These tests run on each system, with a count that the test gives.
- [ ] On macOS, with `config.ACLEntries`: a `config.yaml` with the mode `0644` and the entry
      `everyone allow write` is refused; one with `user:nobody allow append` is refused; one with only
      `everyone deny delete` is refused; one whose ACL has no entries gives its settings; a symbolic link to a file
      with an ACL is refused.
- [ ] `svcview` gives `config.ACLEntries` to `config.Read` before it opens the screen.
- [ ] e2e test, which runs only on macOS and skips in Docker: a `config.yaml` with the mode `0644` and
      `/bin/chmod +a "everyone allow write"` stops svcview with `EXIT:1` and an error that names the file, before
      the screen opens. After `/bin/chmod -N`, the same file opens the screen.
- [ ] `docs/e2e-tests.md` says that the ACL test runs only with `task test:e2e:local` on a Mac, and why.

**Affected Files/Modules:**
- `internal/config/config.go` — `Read` takes the count of the ACL entries, and refuses a file with an ACL
- `internal/config/config_test.go` — the tests of the refusal, with a count that the test gives, and with real ACLs
  on macOS
- `internal/cli/root.go` — give `config.ACLEntries` to `config.Read`
- `e2e/tests/config.test.ts` — the macOS-only e2e test
- `docs/e2e-tests.md` — the test that runs only on a Mac

**Patterns to Follow:**
- `internal/config/config.go:27-54`
- `internal/config/config_test.go:212-266`
- `internal/cli/root.go:52-66`
- `e2e/tests/config.test.ts:47-58`
- `docs/e2e-tests.md:143-203`
- `task:1`

**Testable:** Yes — through `config.Read` with a count that the test gives on each system, with real ACLs on macOS,
and in an e2e test on a Mac.

**Certainty:** medium — the check of the owner and the mode (`config.go:38-45`) and its tests (`config_test.go:212-266`)
are the precedent, but no check takes a function of the system from its caller, and no e2e test runs on one system
only.

**Blast radius:** high — the refusal of a config file that another user can write through an ACL; from the status
command on, that user could run commands as the user of svcview.

**Verification:** `mise exec -- task test:unit` passes on this Mac with the macOS tests run; `task test:e2e:local`
passes on this Mac with the ACL test run; `task test:e2e` passes in Docker with the ACL test skipped.

**Depends on:** Task 1

### Task 3: Read the status command of a group

**Behavior:** A group can have the setting `status`, with `command`, `every` and `timeout`. `command` is a list of
one or more texts: the program, then its arguments. Each text is a Go template that can use the template values
`{{.Name}}`, `{{.Env.X}}`, `{{.StdoutPath}}`, `{{.StderrPath}}` and `{{.PlistPath}}`. `every` is a Go duration more
than 0, `2s` by default, and `timeout` is a Go duration more than 0, `1s` by default. Each error in `status` stops
svcview at start, and the error names the file and the group. svcview runs no command.

**Acceptance Criteria:**
- [ ] A group with `status` gives a status command with its program and arguments, its `every` and its `timeout`.
      Without `every` or `timeout`, the default applies.
- [ ] A group with no `status` has no status command.
- [ ] Each text of `command` can use the five template values, and text with no template value is valid too.
- [ ] Each of these gives an error that names the file and the group, and no settings: a text of `command` that is
      not a Go template; a text that uses a value other than the five, for example `{{.Label}}`, or `{{.Env}}`
      with no name after it; a `command` that is not a list of texts, or that is empty, or whose program is empty;
      a `status` with no `command`; an `every` or a `timeout` that is not a Go duration more than 0; a `status` that
      is not a set of settings; a setting in `status` that svcview does not know, for example `stale_after`; a
      setting that `status` sets two times.
- [ ] e2e test: a `config.yaml` whose group has a `status.command` with an invalid template stops svcview with
      `EXIT:1` and an error that names the file and the group, before the screen opens.

**Affected Files/Modules:**
- `internal/status/command.go` — new: the status command of a group, and the check of its templates
- `internal/status/command_test.go` — new: the tests of the status command
- `internal/config/group.go` — read `status` of a group
- `internal/config/config_test.go` — the tests of `status` through `config.Read`
- `e2e/tests/config.test.ts` — the invalid template

**Patterns to Follow:**
- `internal/config/group.go:34-66`
- `internal/config/config.go:98-105`
- `internal/config/config_test.go:348-432`
- `e2e/tests/config.test.ts:16-28`

**Testable:** Yes — through `status.NewCommand`, through `config.Read` with a `config.yaml` in `t.TempDir()`, and in
an e2e test.

**Certainty:** medium — `readGroup` (`group.go:34-66`) and the duration of `refresh` (`config.go:98-105`) are the
precedent, but no setting holds a set of settings in a group, and no code checks the values that a Go template uses.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2

### Task 4: Describe the refusal of a config file with an ACL in docs/cli.md

**Behavior:** `docs/cli.md` says that svcview refuses `config.yaml` when the file has an ACL, why it refuses each
ACL entry, how to see the ACL and how to remove it. It no longer says that svcview does not check the ACL.

**Acceptance Criteria:**
- [ ] The section "A config file that another user can write" says that svcview refuses `config.yaml` when its ACL
      has one or more entries, whatever the entries allow or deny, and why: an ACL can let another user write a file
      whose mode is `0644`, and svcview cannot tell an entry of the owner from an entry of another user.
- [ ] It says that `/bin/ls -le` shows the ACL and `chmod -N` removes it, and that an ACL with no entries is
      accepted.
- [ ] It says that on Linux the mode shows a POSIX ACL entry that lets another user write the file, so the check of
      the mode refuses that file.
- [ ] No text of `docs/cli.md` says that svcview does not check the ACL, or that svcview reads a file whose ACL lets
      another user write it.
- [ ] The list of errors names a file that has an ACL.

**Affected Files/Modules:**
- `docs/cli.md` — the ACL in the section on a config file that another user can write, and in the errors

**Patterns to Follow:**
- `docs/cli.md:170-212`

**Testable:** No — a document.

**Certainty:** high — the section `docs/cli.md:190-212` is the precedent, and this task changes its paragraph on
the ACL.

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the behaviour of `task cli` on this Mac with a `config.yaml` that
has an ACL.

**Depends on:** Task 2

## Summary

- 4 tasks. The check comes first: Task 1 reads the ACL of an open file, and Task 2 refuses a config file with an ACL
  and wires the check into svcview. Only then does Task 3 add `status`, the first setting that holds a command, so
  no commit has a command setting without the ACL check. The document comes last.
- Covers, of the criterion on template values, the five values that an argument can use and the stop at start for
  an invalid template (Task 3). The run of the command, the values of the plist, and the status on the screen are
  outside this change (Boundaries). Closes the ACL gap of the US-002 criterion "svcview refuses to start when
  another user can write the config file" (Tasks 1, 2 and 4).
- Task 1 is `low` certainty: the first raw system call and the first build constraint of the repo. Tasks 1 and 2
  are `high` blast radius: the read and the refusal decide whether another user of the Mac can put commands into
  the config file.
- The planner chose these details, which the story does not give: any ACL entry refuses the file; a count that
  cannot be read refuses the file; an ACL with no entries is accepted; the template check refuses a value other than
  the five at start; `stale_after` and `actions` stop svcview as unknown settings; the program of `command` can use
  template values too.
