## Contents
1. Task 1: Refuse a config file that is not a regular file
2. Task 2: Read the groups of each `*.yaml` file in `conf.d` after `config.yaml`
3. Task 3: Refuse a `refresh` in a file of `conf.d`
4. Task 4: Refuse a group whose name a group of an earlier config file has
5. Task 5: Describe the files of `conf.d` in docs/cli.md

## Story Reference

Story: `user-stories/launchd-tui.md`, US-005 "Add a group with one file in `conf.d`" (`launchd-tui/US-005`).
Proposal: `docs/proposal-launchd-tui.md`, the sections "Config Files" and "Settings of a Group".

This change lets the installer of a program add a group to svcview with one file, with no edit of the main config
of the user. svcview reads `config.yaml` first, and then each `*.yaml` file in `conf.d/` of the same config folder,
in name order. It joins the `groups` of all files in file order: the groups of `config.yaml` first, then the groups
of each file of `conf.d`. A service that the groups of two files match is in the group of the earlier file. A file of
`conf.d` works when no `config.yaml` exists. When the uninstaller deletes its file, the group is not in the list at
the next start, and its services go to `Other`.

A file of `conf.d` holds status commands that svcview runs as its user, as `config.yaml` does. So each file passes
the check that `config.yaml` passes, in the same code: svcview opens the file, and it refuses the open file when it is
not a regular file, when its owner is not the user of svcview, when its group or other users can write it, or when
it has an ACL with one or more entries. A symbolic link counts with the file that it points to. The error names the
file.

Only `config.yaml` can set `refresh`. A `refresh` in a file of `conf.d` stops svcview at start, and the error names
the file. Two groups with the same name in two files stop svcview at start, and the error names the file and the
group.

The story criteria that this change satisfies:

- svcview reads `config.yaml` first, and then each `*.yaml` file in `conf.d/` in the same folder, in name order.
  svcview joins the `groups` of all files, in file order.
- A file in `conf.d/` works when no `config.yaml` exists.
- When the uninstaller deletes its file, the group is not on the screen at the next start of svcview, and its
  services go to `Other`.
- Two groups with the same name in two files stop svcview at start. The error names the file and the group.
- A `refresh` in a file in `conf.d/` stops svcview at start, because only `config.yaml` can set `refresh`. The error
  names the file.
- svcview refuses to start when another user can write a file in `conf.d/`. The error names the file.
- `docs/cli.md` gives an example of a file in `conf.d/` that an installer writes.

**The rules for the entries of `conf.d`.** The story and the proposal say "each `*.yaml` file in `conf.d/`, in name
order". The planner chose these rules for each other case:

- svcview reads each entry whose name ends in `.yaml`, in lower case, and does not start with a dot. It skips each
  other entry, whatever it is: for example `worker.yml`, `worker.yaml.off`, `worker.YAML`, a folder `old`, and
  `.worker.yaml`. The read of the plists has the same rule for its extension. A name that starts with a dot is
  skipped because programs leave such files next to a file: `._worker.yaml` is the AppleDouble file that macOS
  writes on a volume with no extended attributes, `.#worker.yaml` is the lock link of Emacs, which points to no file,
  and an installer can write `.worker.yaml` before it renames it.
- Name order is the byte order of the names: `10-a.yaml` comes before `2-a.yaml`, and `B.yaml` before `a.yaml`.
- An entry that svcview reads must be a regular file after svcview follows a symbolic link. A folder named
  `old.yaml`, a named pipe, a socket, or a device stops svcview at start, and the error names the entry. svcview does
  not wait on a named pipe that no program writes. The same rule applies to `config.yaml`, because both pass one
  check. A `config.yaml` that is a folder already stops svcview, and a `config.yaml` that is a named pipe makes
  svcview wait with no end, also after `ctrl+c`.
- A symbolic link to no file counts as no file, as for `config.yaml`: svcview skips it. A file that an uninstaller
  deletes after svcview lists `conf.d` and before svcview opens the file counts the same way.
- A `conf.d` that is a file, not a folder, stops svcview at start, and the error names `conf.d`. So does a `conf.d`
  that svcview cannot list. A `conf.d` that is a symbolic link to a folder counts as that folder. No `conf.d`, or a
  symbolic link to no folder, counts as a `conf.d` with no files.

**The join of the groups.** The groups of each config file join the groups of the files before it in one place, and
each check that spans the groups of all files belongs in that place: in this change, the check that two groups do not
have the same name. A group knows the config file that sets it, so an error about a group can name its file also
after the join.

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
- svcview reads the config files only at start. A file that an installer adds, changes, or deletes while svcview runs
  takes effect at the next start.
- svcview checks each file that it reads. It does not check the owner, the mode, or the ACL of `conf.d` or of the
  config folder, as it does not check the folder of `config.yaml`.
- svcview reads no folder inside `conf.d`, and no file inside such a folder.
- svcview reads no other extension than `.yaml` in lower case. It reads no `.yml`.
- A file of `conf.d` can set only `groups`. It cannot set `refresh`, and it cannot change, replace, or remove a group
  of another file.
- No setting or flag names another folder than `conf.d`, and no setting turns off a file of `conf.d`.
- The screen does not show which config file sets a group.
- svcview has no command that writes or deletes a file of `conf.d`. The installer and the uninstaller of the program
  do that.
- A group has no `stale_after` in `status`, and no `actions`. Each of them stops svcview at start, as a setting that
  svcview does not know.

**Deferred:**
- No check of the keys of the groups of all files: a group has no `actions` yet.

## Codebase Context

- `internal/config/config.go:28-62`: `config.Read(folder, uid, aclEntries)` opens `config.yaml` in the config
  folder, counts a file that does not exist as no file (`config.go:31-34`), checks the open file (`config.go:39-53`:
  the owner and the mode from `Stat` of the open file, then the count of `aclEntries`), reads it, and gives the
  content to `(*Settings).read` (`config.go:64-94`). It wraps each error of a setting as
  `the config file <path>: ...`. The check is written once, inline in `Read`, so it has no place that a second file
  can pass today.
- `(*Settings).read` reads each YAML document of a file, refuses a top-level setting that the file sets two times,
  and gives each setting to `(*Settings).set` (`config.go:96-104`), which knows `refresh` and `groups` and refuses each
  other setting. `setGroups` (`config.go:115-135`) appends each group to `Settings.Groups`, and refuses a name that
  an earlier group of the same file has, with the line of each.
- `Settings.GroupOf` (`config.go:137-144`) gives the first group whose match matches, else `config.Other`.
  `internal/tui/screen.go:96` puts `settings.Groups` and then `Other` in the order of the list, and the screen finds
  the group of a row with `slices.Index` on `[]config.Group`, so `config.Group` must stay comparable. The screen needs
  no change: it shows the groups of `Settings` in their order.
- `internal/cli/root.go:52-67`: `showScreen` calls `config.Read` with `config.Folder(home, XDG_CONFIG_HOME)` and
  `config.ACLEntries` before the screen opens, and an error stops svcview with exit code 1. It needs no change:
  `conf.d` is in the folder that it gives.
- `internal/config/acl_darwin.go`: `config.ACLEntries(file)` counts the ACL entries of an open file; `acl_other.go`
  gives 0 on other systems. A test that changes a real ACL runs only on macOS, uses `/bin/chmod`, and removes the ACL
  in its cleanup through the helper `chmod` of `internal/config/acl_test.go:26-35` (`tasks/learnings.md`, "An ACL test
  removes the ACL before the cleanup of its folder").
- `internal/launchd/plist_folder.go:15-45` lists a folder with `os.ReadDir`, keeps the names with one extension
  (case-sensitive), and counts a folder that does not exist as empty. It skips a file that it cannot stat. The read of
  `conf.d` refuses such a file instead, because a skipped file would hide a refusal.
- A probe during planning, on this Mac (Go 1.27.4, darwin arm64), for the entries of `conf.d`:
  - `os.ReadDir` gives the entries sorted in the byte order of the names: `.hidden.yaml`, `10-a.yaml`, `2-a.yaml`,
    `A.yaml`, `b.yaml`. The type of an entry is `L` for a symbolic link, `d` for a folder, and `p` for a named pipe.
  - `filepath.Glob` with `*.yaml` gives `.hidden.yaml` too.
  - `os.Open` of a named pipe with no writer does not return: it still waited after 500 ms. An open with
    `O_NONBLOCK` returned at once, and `Stat` of the open file gave the mode `prw-r--r--`. A regular file opened
    with `O_NONBLOCK` read as usual.
  - `os.Open` of a folder succeeds, `Stat` gives a folder, and the read fails with `is a directory`.
  - `os.Open` of a symbolic link to no file gives `fs.ErrNotExist`.
  - `os.ReadDir` of a file gives `not a directory`; of a folder that does not exist, or of a symbolic link to no
    folder, `fs.ErrNotExist`; of a folder with the mode `000`, `permission denied`.
- `cmd/svcview/main.go` catches `SIGINT`, `SIGHUP`, and `SIGTERM` with `signal.NotifyContext`, and `config.Read`
  takes no context, so `ctrl+c` does not stop svcview while an open waits on a named pipe.
- Tests: `TestSettings` in `internal/config/config_test.go` has the helpers `folderWith` and `readSettings`
  (`config_test.go:28-41`), which write `config.yaml` into `t.TempDir()`, and the fake ACL counts `noACL` and
  `aclWithEntries` (`config_test.go:17-26`). The tests of the check of `config.yaml` are at `config_test.go:200-348`:
  a folder, a file that svcview cannot read, the mode, the owner, a symbolic link, the ACL count, and on macOS a real
  ACL. The order of the groups is at `config_test.go:357-362`, two groups with one name at `config_test.go:630-638`,
  and a service that two groups match at `config_test.go:680-686`. A test whose error names a path in `t.TempDir()`
  checks a text in quotes, because the name of the subtest is in the path (`tasks/learnings.md`).
- e2e: `writeConfig(configFolder, content, mode)` (`e2e/testUtils.ts:80-86`) writes `config.yaml` with a mode, and
  `openCli` unsets `XDG_CONFIG_HOME`. `e2e/tests/config.test.ts:48-59` tests a file that all users can write, and
  `config.test.ts:109-137` the order of the groups on the screen with `listLines`. The Docker tests run as root, so
  root owns each file that a test writes. An error that names a path continues on the next line, so a test joins the
  lines of the screen first. `docs/e2e-tests.md:143-186` describes the config helper.
- `docs/cli.md:148-176` describes the config file and its settings, `docs/cli.md:178-207` the groups,
  `docs/cli.md:428-456` the errors, and `docs/cli.md:458-495` the refusal of a file that another user can write. It
  says that svcview reads `config.yaml`, and that svcview does not check the folders that hold the file.
- Words: "config file" is `config.yaml` or a file of `conf.d`, and each error says "the config file <path>"; "config
  folder" is `$XDG_CONFIG_HOME/svcview` or `~/.config/svcview`; "join" is what svcview does to the groups of all files,
  as the proposal says; "file order" is `config.yaml`, then the files of `conf.d` in name order.
- Run procedure (`tasks/learnings.md`): run Go and clerk through `mise exec --`; run `gofmt -l internal` before
  `clerk finish`; remove `e2e/package-lock.json` after `task test:e2e:local`.

## Tasks

### Task 1: Refuse a config file that is not a regular file

**Behavior:** svcview checks a config file in one place. It opens the file with no wait on a named pipe, and it
refuses the open file when it is not a regular file, as well as for the owner, the mode, and the ACL. A
`config.yaml` that is a folder, a named pipe, a socket, or a device stops svcview at start, and the error names the
file and says that it is not a regular file. A symbolic link counts with the file that it points to. Each other case
of the check of `config.yaml` gives the same result as before.

**Acceptance Criteria:**
- [x] A `config.yaml` that is a folder gives an error that names the file and says that it is not a regular file,
      and no settings.
- [x] A `config.yaml` that is a named pipe with no writer gives the same error at once: `config.Read` returns and
      does not wait for a writer.
- [x] A `config.yaml` that is a symbolic link to a folder, or to a named pipe, gives the same error. A symbolic link
      to a regular file still counts with the owner, the mode, and the ACL of that file.
- [x] Each test of the check of `config.yaml` at `internal/config/config_test.go:200-348` passes, with no change to
      what it checks.
- [x] The check of an open config file (a regular file, the owner, the mode, and the ACL) and the read of its content
      are in one place in `internal/config`, which any config file can pass; `config.Read` uses it for `config.yaml`.

**Affected Files/Modules:**
- `internal/config/config.go` — the one check of a config file, with the refusal of a file that is not a regular
  file and an open that does not wait on a named pipe
- `internal/config/config_test.go` — the tests of a folder, a named pipe, and a symbolic link to each

**Patterns to Follow:**
- `internal/config/config.go:28-62`
- `internal/config/config_test.go:200-209`
- `internal/config/config_test.go:260-278`

**Testable:** Yes — through `config.Read` with a folder, a named pipe, and symbolic links in `t.TempDir()`.

**Certainty:** medium — the check of the open file (`config.go:39-53`) is the precedent, but no open in the repo
must avoid the wait of a named pipe; the probe in Codebase Context gives the behaviour of the open.

**Blast radius:** high — this is the one check that decides whether svcview reads a config file, whose status
commands it runs as its user.

**Verification:** `mise exec -- task test:unit` passes on this Mac with the macOS tests run, and
`GOOS=linux mise exec -- go vet -tags=unit ./...` passes.

**Depends on:** None

### Task 2: Read the groups of each `*.yaml` file in `conf.d` after `config.yaml`

**Behavior:** `config.Read` reads `config.yaml`, and then each entry of `conf.d/` in the config folder whose name ends
in `.yaml` and does not start with a dot, in the byte order of the names. Each file passes the check of Task 1 before
svcview reads it. The groups of each file join the groups of the files before it, in one place, so the list shows the
groups of `config.yaml`, then the groups of each file of `conf.d`, then `Other`. A service that the groups of two
files match is in the group of the earlier file. With no `config.yaml`, the refresh is `2s` and the groups are those
of `conf.d`. A file that is gone at the next start gives no group, so its services are in `Other`. Each error in a
file of `conf.d` names that file. The rules for the entries of `conf.d` are those of the Story Reference.

**Acceptance Criteria:**
- [ ] The groups of `config.yaml` come first, then the groups of each file of `conf.d` in the byte order of the names
      (`10-b.yaml` before `2-a.yaml`, `B.yaml` before `a.yaml`), and the groups of each file keep the order of that
      file.
- [ ] A file of `conf.d` gives its groups when no `config.yaml` exists, and the refresh is `2s`.
- [ ] After the test deletes the file of `conf.d`, a new read has no group of that file, and the services of the
      group are in `Other`.
- [ ] A service that a group of `config.yaml` and a group of a file of `conf.d` match is in the group of
      `config.yaml`. A service that the groups of two files of `conf.d` match is in the group of the earlier file.
- [ ] An entry whose name does not end in `.yaml`, or whose name starts with a dot, is skipped, whatever the entry
      is: a file that is not valid YAML, a file that all users can write, or a folder.
- [ ] An entry whose name ends in `.yaml` and that is not a regular file stops svcview with the error of Task 1,
      which names the entry. svcview does not wait on a named pipe there.
- [ ] An entry that is a symbolic link to no file is skipped. A symbolic link to a file counts with that file.
- [ ] With no `conf.d`, or with a `conf.d` that is a symbolic link to no folder, the settings are those of
      `config.yaml`. A `conf.d` that is a symbolic link to a folder gives the files of that folder.
- [ ] A `conf.d` that is a file, or that svcview cannot list, gives an error that names `conf.d`, and no settings.
      The test of a `conf.d` that svcview cannot list skips when it runs as root.
- [ ] Each refusal of the check of `config.yaml` refuses a file of `conf.d` too, with an error that names the file
      of `conf.d` and no settings: each test of the check at `config_test.go:200-348` that refuses a file has a
      counterpart for a file of `conf.d`. A file of `conf.d` that passes the check gives its groups.
- [ ] On macOS, with `config.ACLEntries`: a file of `conf.d` with the mode `0644` and one ACL entry is refused, and
      so is a symbolic link in `conf.d` to a file with an ACL entry. These tests skip on other systems.
- [ ] An error in a setting of a file of `conf.d`, for example a setting that svcview does not know or a `match` that
      is not a Go regular expression, names that file.
- [ ] e2e: with no `config.yaml`, a `conf.d/worker.yaml` with the group `Workers` makes the list show `Workers` with
      its service, before `Other`.
- [ ] e2e: with a group in `config.yaml` and a group in each of two files of `conf.d`, the list shows the groups in
      file order, then `Other`.
- [ ] e2e: after the test deletes `conf.d/worker.yaml`, the next start of svcview shows no line `Workers`, and the
      service of `Workers` is under `Other`.
- [ ] e2e: a file of `conf.d` that all users can write stops svcview with `EXIT:1` and an error that names the file,
      before the screen opens.
- [ ] `docs/e2e-tests.md` describes the helper that writes a file of `conf.d`.

**Affected Files/Modules:**
- `internal/config/config.go` — the list of the files of `conf.d`, the read of each through the check of Task 1,
  and the join of their groups
- `internal/config/config_test.go` — the tests of the order, the join, the entries of `conf.d`, and the check of each
  file of `conf.d`
- `e2e/testUtils.ts` — a helper that writes a file of `conf.d` with a mode
- `e2e/tests/config.test.ts` — the groups of `conf.d` on the screen, the deleted file, and the refusal of a file
  that all users can write
- `docs/e2e-tests.md` — the helper of a file of `conf.d`

**Patterns to Follow:**
- `internal/config/config.go:28-62`
- `internal/launchd/plist_folder.go:15-45`
- `internal/config/config_test.go:200-348`
- `internal/config/config_test.go:357-362`
- `internal/config/config_test.go:680-686`
- `e2e/testUtils.ts:80-86`
- `e2e/tests/config.test.ts:48-59`
- `e2e/tests/config.test.ts:109-137`
- `docs/e2e-tests.md:143-186`
- `task:1`

**Testable:** Yes — through `config.Read` with a config folder in `t.TempDir()`, and in e2e tests that start svcview.

**Certainty:** medium — `config.Read` (`config.go:28-62`) and the read of the plist folder (`plist_folder.go:15-45`)
are the precedents, but no code reads more than one config file, joins what several files set, or stops on an entry
of a folder instead of skipping it.

**Blast radius:** high — this widens the set of files whose status commands svcview runs as its user, so each file
must pass the check of `config.yaml`.

**Verification:** `mise exec -- task test:unit` passes on this Mac with the macOS tests run; `task test:e2e` passes in
Docker; `task test:e2e:local` passes on this Mac.

**Depends on:** Task 1

### Task 3: Refuse a `refresh` in a file of `conf.d`

**Behavior:** Only `config.yaml` can set `refresh`. A file of `conf.d` that sets `refresh` stops svcview at start. The
error names the file and the setting, and says that only `config.yaml` can set it.

**Acceptance Criteria:**
- [ ] A file of `conf.d` with `refresh` gives an error that names the file, `"refresh"`, and `config.yaml`, and no
      settings: with a `config.yaml` that sets `refresh`, with one that does not, and with no `config.yaml`.
- [ ] The same error comes for a `refresh` in a later YAML document of the file, and for a `refresh` whose value is
      not a duration.
- [ ] `refresh` in `config.yaml` works as before, also when files of `conf.d` exist.
- [ ] A setting that svcview does not know in a file of `conf.d` gives the error that it gives in `config.yaml`,
      and the error names the file of `conf.d`.
- [ ] e2e: a `conf.d/worker.yaml` with `refresh: 5s` stops svcview with `EXIT:1` and an error that names the file,
      before the screen opens.

**Affected Files/Modules:**
- `internal/config/config.go` — the refusal of `refresh` in a file of `conf.d`
- `internal/config/config_test.go` — the tests of the refusal
- `e2e/tests/config.test.ts` — the refusal at start

**Patterns to Follow:**
- `internal/config/config.go:96-104`
- `internal/config/config_test.go:143-156`
- `e2e/tests/config.test.ts:17-29`
- `task:2`

**Testable:** Yes — through `config.Read` with a config folder in `t.TempDir()`, and in an e2e test.

**Certainty:** medium — `(*Settings).set` (`config.go:96-104`) refuses a setting with an error that names the file
and the setting, but no setting so far is valid in one config file and refused in another.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2

### Task 4: Refuse a group whose name a group of an earlier config file has

**Behavior:** The name of a group is unique across all config files. A group whose name a group of an earlier file
has stops svcview at start. The error names the file and the line of the second group, the group, and the file of
the first group. The check is in the one place where the groups of each file join the groups of the files before it.
Two groups with the same name in one file keep the error that they give today.

**Acceptance Criteria:**
- [ ] A group of a file of `conf.d` whose name a group of `config.yaml` has gives an error that names the file of
      `conf.d`, the group, and `config.yaml`, and no settings.
- [ ] A group of a file of `conf.d` whose name a group of an earlier file of `conf.d` has gives an error that names
      the later file, the group, and the earlier file, and no settings.
- [ ] Two names that differ only in case, in two files, are two groups.
- [ ] Two groups with the same name in one file give the error at `config_test.go:630-638`, with the line of each.
- [ ] e2e: a group `Workers` in `config.yaml` and a group `Workers` in `conf.d/worker.yaml` stop svcview with
      `EXIT:1` and an error that names `conf.d/worker.yaml` and `group "Workers"`, before the screen opens.

**Affected Files/Modules:**
- `internal/config/config.go` — the check of the names in the join of the groups
- `internal/config/group.go` — a group knows the config file that sets it
- `internal/config/config_test.go` — the tests of two groups with one name in two files
- `e2e/tests/config.test.ts` — the refusal at start

**Patterns to Follow:**
- `internal/config/config.go:115-135`
- `internal/config/config_test.go:630-638`
- `internal/config/config_test.go:650-655`
- `e2e/tests/config.test.ts:83-98`
- `task:2`

**Testable:** Yes — through `config.Read` with a config folder in `t.TempDir()`, and in an e2e test.

**Certainty:** medium — `setGroups` (`config.go:115-135`) refuses a name that an earlier group of the same file has,
but no check spans the groups of two files, and this error must name a file other than the one that svcview reads.

**Blast radius:** low

**Verification:** `mise exec -- task test:unit` passes, and `task test:e2e` passes.

**Depends on:** Task 2

### Task 5: Describe the files of `conf.d` in docs/cli.md

**Behavior:** `docs/cli.md` describes `conf.d`: which files svcview reads and in which order, how it joins their
groups, what a file of `conf.d` can set, the check of each file, and the errors. It gives an example of a file of
`conf.d` that an installer writes, and what the installer and the uninstaller do.

**Acceptance Criteria:**
- [ ] The section "The config file" says that svcview reads `config.yaml` and then each `*.yaml` file in `conf.d/`
      of the config folder, in the byte order of the names, with an example of that order; that it skips a name that
      starts with a dot or does not end in `.yaml`; that a file of `conf.d` works with no `config.yaml`; and that a
      change to any config file takes effect at the next start.
- [ ] It says that svcview joins the groups of all files in file order, that a service that the groups of two files
      match is in the group of the earlier file, that the name of a group must be unique across all files, and that
      only `config.yaml` can set `refresh`.
- [ ] It gives an example of `conf.d/worker.yaml` that the installer of a program writes, with a group and a status
      command, and with only settings that svcview knows.
- [ ] It says what the installer does: it writes the file into `conf.d` of the config folder of the user, as svcview
      finds that folder from `XDG_CONFIG_HOME`; it writes the file as the user, not as root, with no write permission
      for the group and other users; and it writes the file under a name that svcview skips and then renames it. It
      says that the uninstaller deletes the file, and that the services of the group are then in `Other` at the next
      start.
- [ ] The list of errors names a `refresh` in a file of `conf.d`, two groups with the same name in two files, a
      config file that is not a regular file, and a `conf.d` that is not a folder or that svcview cannot list.
- [ ] The section "A config file that another user can write" says that svcview checks each file of `conf.d` as it
      checks `config.yaml`, and that it does not check `conf.d` or the config folder.
- [ ] No text of `docs/cli.md` says that `config.yaml` is the only file that svcview reads.

**Affected Files/Modules:**
- `docs/cli.md` — `conf.d` in the config file, the groups, the errors, and the refusal of a file that another user
  can write

**Patterns to Follow:**
- `docs/cli.md:148-176`
- `docs/cli.md:178-207`
- `docs/cli.md:428-456`
- `docs/cli.md:458-495`

**Testable:** No — a document.

**Certainty:** high — the sections `docs/cli.md:148-207` and `docs/cli.md:428-495` are the precedent, and this task
extends them to the files of `conf.d`.

**Blast radius:** low

**Verification:** A read of `docs/cli.md` against the behaviour of `task cli` on this Mac with a
`conf.d/worker.yaml` and no `config.yaml`, and against the errors that the tests of Tasks 1 to 4 check.

**Depends on:** Task 3, Task 4

## Summary

- 5 tasks. The one check of a config file comes first (Task 1), so the read of `conf.d` (Task 2) uses it from its
  first commit, and no commit reads a file of `conf.d` without the check of `config.yaml`. Task 2 also makes the one
  place where the groups of the files join; Tasks 3 and 4 add the two refusals that depend on which file sets a
  setting. The document comes last.
- Covers all 7 story criteria: the order and the join (Task 2, described in Task 5); a file of `conf.d` with no
  `config.yaml` (Task 2); the deleted file (Task 2); two groups with one name in two files (Task 4); `refresh` in a
  file of `conf.d` (Task 3); a file of `conf.d` that another user can write (Tasks 1 and 2); the example of an
  installer file (Task 5).
- Tasks 1 and 2 are `high` blast radius: they decide which files svcview reads before it runs their status commands
  as its user. No task is `low` certainty: each code task varies the read of `config.yaml`, and the probe in Codebase
  Context answers how an open behaves on a named pipe. The code tasks are `medium`, and the document is `high`.
- The planner chose these details, which the story does not give: the rules for the entries of `conf.d` in the Story
  Reference (lower-case `.yaml`, no name that starts with a dot, byte order, a regular file after a symbolic link, a
  symbolic link to no file skipped, a `conf.d` that is not a folder refused); the check of a regular file applies to
  `config.yaml` too; svcview does not check `conf.d` or the config folder; the error of two groups with one name also
  names the file of the first group; a group knows the config file that sets it.
