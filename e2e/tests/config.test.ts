import { execFileSync } from 'node:child_process'
import { join } from 'node:path'
import { describe, expect, it, onTestFinished, vi } from 'vitest'
import { addPlist, buildTag, homeWithPlists, openCli, runCli, scratchDir, writeConfig } from '../testUtils'

function listLines(screen: string): string[] {
  const lines = screen.split('\n')
  const list: string[] = []
  for (const line of lines.slice(lines.findIndex((line) => line.includes('╭ Services')) + 1)) {
    if (line.startsWith('╰')) break
    list.push(line.split('│')[1].trim())
  }
  return list
}

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
