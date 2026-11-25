import { execFileSync } from 'node:child_process'
import { rmSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it, onTestFinished, vi } from 'vitest'
import {
  addPlist,
  buildTag,
  homeWithPlists,
  listLines,
  openCli,
  runCli,
  scratchDir,
  writeConfD,
  writeConfig,
} from '../testUtils'

describe('the config file', () => {
  it('an unknown setting in $XDG_CONFIG_HOME/svcview/config.yaml stops svcview with the exit code 1 and an error that names the file, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const xdgConfigHome = scratchDir()
    const path = writeConfig(join(xdgConfigHome, 'svcview'), 'colour: blue\n')

    const session = await openCli(scratchDir(), [], { HOME: home, XDG_CONFIG_HOME: xdgConfigHome })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('"colour"')
    expect(printed).not.toContain('com.example.worker')
  })

  it('with no XDG_CONFIG_HOME, an unknown setting in $HOME/.config/svcview/config.yaml stops svcview with the exit code 1 and an error that names the file, also when the test run sets XDG_CONFIG_HOME', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const path = writeConfig(join(home, '.config', 'svcview'), 'colour: blue\n')
    const xdgConfigHomeOfTheRun = scratchDir()
    writeConfig(join(xdgConfigHomeOfTheRun, 'svcview'), 'refresh: 5s\n')
    vi.stubEnv('XDG_CONFIG_HOME', xdgConfigHomeOfTheRun)
    onTestFinished(() => vi.unstubAllEnvs())

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('"colour"')
    expect(printed).not.toContain('com.example.worker')
  })

  it('a config.yaml that all users can write stops svcview with the exit code 1 and an error that names the file, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const path = writeConfig(join(home, '.config', 'svcview'), 'refresh: 5s\n', 0o666)

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('another user can write')
    expect(printed).not.toContain('com.example.worker')
  })

  it.skipIf(process.platform !== 'darwin')(
    'on macOS, a config.yaml with the mode 0644 and an ACL entry stops svcview with the exit code 1 and an error that names the file, before the screen opens, and opens the screen after chmod -N',
    async () => {
      const home = homeWithPlists(['com.example.worker'])
      const path = writeConfig(join(home, '.config', 'svcview'), 'refresh: 5s\n')
      execFileSync('/bin/chmod', ['+a', 'everyone allow write', path])

      const refused = await openCli(scratchDir(), [], { HOME: home })

      await refused.waitForText('EXIT:1')
      const printed = (await refused.text()).replaceAll('\n', '')
      expect(printed).toContain(path)
      expect(printed).toContain('chmod -N')
      expect(printed).not.toContain('com.example.worker')

      execFileSync('/bin/chmod', ['-N', path])
      const opened = await openCli(scratchDir(), [], { HOME: home })

      await opened.waitForText('com.example.worker')
    },
  )

  it('a status command with an invalid template stops svcview with the exit code 1 and an error that names the file and the group, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const path = writeConfig(
      join(home, '.config', 'svcview'),
      "groups:\n  - name: Workers\n    match: worker\n    status:\n      command: [cat, '{{.Env.WORKER_STATUS_FILE}']\n",
    )

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('group "Workers"')
    expect(printed).toContain('{{.Env.WORKER_STATUS_FILE}')
    expect(printed).not.toContain('com.example.worker')
  })

  it('a valid config.yaml opens the screen', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfig(join(home, '.config', 'svcview'), 'refresh: 5s\n')

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('com.example.worker')
  })

  it('the list shows the groups of config.yaml in the order of the file, then the group Other, with the services of each group sorted by name', async () => {
    const home = homeWithPlists([
      'homebrew.mxcl.redis',
      'com.example.worker.02',
      'com.apple.a',
      'homebrew.mxcl.postgres',
      'com.example.worker.01',
    ])
    writeConfig(
      join(home, '.config', 'svcview'),
      'groups:\n  - name: Workers\n    match: worker\n  - name: Databases\n    match: redis|postgres\n',
    )

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('com.apple.a')
    expect(listLines(await session.text()).slice(0, 10)).toEqual([
      'Workers',
      '▌○ com.example.worker.01',
      '○ com.example.worker.02',
      '',
      'Databases',
      '○ homebrew.mxcl.postgres',
      '○ homebrew.mxcl.redis',
      '',
      'Other',
      '○ com.apple.a',
    ])
  })

  it('a row in a group shows the short name of its service, and the title of the detail pane shows its full name', async () => {
    const home = homeWithPlists(['com.example.worker.02', 'com.apple.a', 'com.example.worker.01'])
    writeConfig(join(home, '.config', 'svcview'), 'groups:\n  - name: Workers\n    match: ^com\\.example\\.worker\\.\n')

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('com.apple.a')
    const screen = await session.text()
    expect(listLines(screen).slice(0, 6)).toEqual(['Workers', '▌○ 01', '○ 02', '', 'Other', '○ com.apple.a'])
    expect(screen).toContain('╭ com.example.worker.01 ─')
  })

  it('with refresh: 1m, a plist that is added while the screen is open does not show within 4 s', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfig(join(home, '.config', 'svcview'), 'refresh: 1m\n')
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('com.example.worker')

    addPlist(home, 'com.example.added')
    await new Promise((done) => setTimeout(done, 4000))

    expect(await session.text()).not.toContain('com.example.added')
  })

  it('svcview version does not read the config file', async () => {
    const home = homeWithPlists([])
    writeConfig(join(home, '.config', 'svcview'), 'colour: blue\n')

    const result = await runCli(scratchDir(), ['version'], { HOME: home })

    expect(result.status).toBe(0)
    expect(result.stdout.trim()).toBe(buildTag())
  })

  it('the help that svcview prints without a terminal does not read the config file', async () => {
    const home = homeWithPlists([])
    writeConfig(join(home, '.config', 'svcview'), 'colour: blue\n')

    const result = await runCli(scratchDir(), [], { HOME: home })

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('print the tag svcview was built from')
  })
})

describe('the files of conf.d', () => {
  it('with no config.yaml, a conf.d/worker.yaml with the group Workers makes the list show Workers with its service, before Other', async () => {
    const home = homeWithPlists(['com.example.worker', 'com.apple.a'])
    writeConfD(join(home, '.config', 'svcview'), 'worker.yaml', 'groups:\n  - name: Workers\n    match: worker\n')

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('com.apple.a')
    expect(listLines(await session.text()).slice(0, 5)).toEqual([
      'Workers',
      '▌○ com.example.worker',
      '',
      'Other',
      '○ com.apple.a',
    ])
  })

  it('the list shows the groups of config.yaml, then the groups of each file of conf.d in the order of their names, then Other', async () => {
    const home = homeWithPlists(['com.example.worker', 'homebrew.mxcl.redis', 'com.example.job', 'com.apple.a'])
    const configFolder = join(home, '.config', 'svcview')
    writeConfig(configFolder, 'groups:\n  - name: Databases\n    match: redis\n')
    writeConfD(configFolder, 'worker.yaml', 'groups:\n  - name: Workers\n    match: worker\n')
    writeConfD(configFolder, 'jobs.yaml', 'groups:\n  - name: Jobs\n    match: job\n')

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('com.apple.a')
    expect(listLines(await session.text()).slice(0, 11)).toEqual([
      'Databases',
      '▌○ homebrew.mxcl.redis',
      '',
      'Jobs',
      '○ com.example.job',
      '',
      'Workers',
      '○ com.example.worker',
      '',
      'Other',
      '○ com.apple.a',
    ])
  })

  it('after the test deletes conf.d/worker.yaml, the next start of svcview shows no line Workers, and the service of Workers is under Other', async () => {
    const home = homeWithPlists(['com.example.worker', 'com.apple.a'])
    const path = writeConfD(join(home, '.config', 'svcview'), 'worker.yaml', 'groups:\n  - name: Workers\n    match: worker\n')
    const installed = await openCli(scratchDir(), [], { HOME: home })
    await installed.waitForText('com.apple.a')
    expect(listLines(await installed.text())).toContain('Workers')

    rmSync(path)
    const uninstalled = await openCli(scratchDir(), [], { HOME: home })

    await uninstalled.waitForText('com.apple.a')
    const lines = listLines(await uninstalled.text())
    expect(lines).not.toContain('Workers')
    expect(lines.slice(0, 3)).toEqual(['Other', '▌○ com.apple.a', '○ com.example.worker'])
  })

  it('a conf.d/worker.yaml with refresh: 5s stops svcview with the exit code 1 and an error that names the file, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const path = writeConfD(join(home, '.config', 'svcview'), 'worker.yaml', 'refresh: 5s\n')

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('only config.yaml can set the setting "refresh"')
    expect(printed).not.toContain('com.example.worker')
  })

  it('a group Workers in config.yaml and a group Workers in conf.d/worker.yaml stop svcview with the exit code 1 and an error that names conf.d/worker.yaml and the group, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const configFolder = join(home, '.config', 'svcview')
    writeConfig(configFolder, 'groups:\n  - name: Workers\n    match: worker\n')
    const path = writeConfD(configFolder, 'worker.yaml', 'groups:\n  - name: Workers\n    match: worker\n')

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('group "Workers"')
    expect(printed).not.toContain('com.example.worker')
  })

  it('a control character in the name of a file of conf.d shows as � in the error, so the name cannot send a sequence to the terminal', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfD(join(home, '.config', 'svcview'), '\x1b[2Jworker.yaml', 'groups: []\n', 0o666)

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain('another user can write the config file')
    expect(printed).toContain('conf.d/�[2Jworker.yaml')
  })

  it('a file of conf.d that all users can write stops svcview with the exit code 1 and an error that names the file, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const path = writeConfD(
      join(home, '.config', 'svcview'),
      'worker.yaml',
      'groups:\n  - name: Workers\n    match: worker\n',
      0o666,
    )

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('another user can write')
    expect(printed).not.toContain('com.example.worker')
  })
})
