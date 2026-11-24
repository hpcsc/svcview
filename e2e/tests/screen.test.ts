import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  addPlist,
  homeWithPlists,
  openCli,
  scratchDir,
  setLaunchctlList,
  setLaunchctlOutput,
  writeConfig,
} from '../testUtils'

describe('the screen', () => {
  it('in a terminal, with no command, shows the domain and the services in LaunchAgents, sorted by name', async () => {
    const home = homeWithPlists(['homebrew.mxcl.redis', 'com.example.worker', 'com.apple.a'])

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('homebrew.mxcl.redis')
    const screen = await session.text()
    expect(screen).toContain('svcview')
    expect(screen).toContain(`gui/${process.getuid?.()}`)
    const rows = ['com.apple.a', 'com.example.worker', 'homebrew.mxcl.redis'].map((name) => screen.indexOf(name))
    expect(rows).not.toContain(-1)
    expect(rows).toEqual([...rows].sort((a, b) => a - b))
  })

  it('q ends svcview with the exit code 0', async () => {
    const session = await openCli(scratchDir(), [], { HOME: homeWithPlists(['com.example.worker']) })
    await session.waitForText('com.example.worker')

    await session.press('q')

    await session.waitForText('EXIT:0')
  })

  it('with stdin not a terminal, prints the help and does not open the screen', async () => {
    const session = await openCli(scratchDir(), ['<', '/dev/null'], { HOME: homeWithPlists(['com.example.worker']) })

    await session.waitForText('EXIT:0')

    const screen = await session.text()
    expect(screen).toContain('print the tag svcview was built from')
    expect(screen).not.toContain('com.example.worker')
  })

  it('with stdout not a terminal, prints the help and does not open the screen', async () => {
    const dir = scratchDir()
    const session = await openCli(dir, ['>', 'output.txt'], { HOME: homeWithPlists(['com.example.worker']) })

    await session.waitForText('EXIT:0')

    expect(readFileSync(join(dir, 'output.txt'), 'utf8')).toContain('print the tag svcview was built from')
    expect(await session.text()).not.toContain('com.example.worker')
  })

  it('with no home folder, says so and exits with 1', async () => {
    const session = await openCli(scratchDir(), [], { HOME: '' })

    await session.waitForText('EXIT:1')

    expect(await session.text()).toContain('$HOME is not defined')
  })

  it('shows a plist that is added while the screen is open, with no key press', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('com.example.worker')

    addPlist(home, 'com.example.added')

    await session.waitForText('com.example.added', { timeout: 5000 })
  })

  it('starts each row with the glyph of its state, and shows the glyph of a new state at the next refresh, with no key press', async () => {
    const home = homeWithPlists(['com.example.running', 'com.example.loaded', 'com.example.not-loaded'])
    setLaunchctlList(home, [{ name: 'com.example.running', pid: 4122 }, { name: 'com.example.loaded' }])
    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('● com.example.running')
    const screen = await session.text()
    expect(screen).toContain('◐ com.example.loaded')
    expect(screen).toContain('○ com.example.not-loaded')

    setLaunchctlList(home, [
      { name: 'com.example.running', pid: 4122 },
      { name: 'com.example.loaded', pid: 4123 },
    ])

    await session.waitForText('● com.example.loaded', { timeout: 5000 })
  })

  it('↓ selects the next row, which then starts with ▌', async () => {
    const session = await openCli(scratchDir(), [], { HOME: homeWithPlists(['com.example.a', 'com.example.b']) })
    await session.waitForText('▌○ com.example.a')

    await session.press('down')

    await session.waitForText('▌○ com.example.b')
    expect(await session.text()).toContain('  ○ com.example.a')
  })

  it('j selects the next row, which then starts with ▌, and k then selects the row before it', async () => {
    const session = await openCli(scratchDir(), [], { HOME: homeWithPlists(['com.example.a', 'com.example.b']) })
    await session.waitForText('▌○ com.example.a')

    await session.press('j')

    await session.waitForText('▌○ com.example.b')
    expect(await session.text()).toContain('  ○ com.example.a')

    await session.press('k')

    await session.waitForText('▌○ com.example.a')
    expect(await session.text()).toContain('  ○ com.example.b')
  })

  it('shows the name, the PID, the uptime and the last exit of the selected service in the detail pane, and the next service after ↓', async () => {
    const home = homeWithPlists(['com.example.a-running', 'com.example.b-killed'])
    setLaunchctlList(home, [
      { name: 'com.example.a-running', pid: 4122 },
      { name: 'com.example.b-killed', pid: 4123, status: -9 },
    ])
    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('PID 4122')
    const screen = await session.text()
    expect(screen).toContain('╭ com.example.a-running ─')
    expect(screen).toContain('● running     PID 4122')
    expect(screen).toContain('up 3h12m      last exit 0')

    await session.press('down')

    await session.waitForText('╭ com.example.b-killed ─')
    const after = await session.text()
    expect(after).toContain('PID 4123')
    expect(after).toContain('last exit SIGKILL')
  })

  it('tab gives the focus to the detail pane, and ↓ then scrolls it to the last of 40 events, with the same service selected', async () => {
    const home = homeWithPlists(['com.example.worker.02'])
    const statusFile = join(scratchDir(), 'status.json')
    const events = Array.from({ length: 40 }, (_, i) => ({
      at: `2026-11-23T09:${String(i).padStart(2, '0')}:00Z`,
      level: 'info',
      text: `event ${String(i + 1).padStart(2, '0')}`,
    }))
    writeFileSync(statusFile, `${JSON.stringify({ version: 1, events })}\n`)
    addPlist(home, 'com.example.worker.01', { WORKER_STATUS_FILE: statusFile })
    writeConfig(
      join(home, '.config', 'svcview'),
      `groups:\n  - name: Workers\n    match: '^com\\.example\\.worker\\.'\n    status:\n      command: [cat, '{{.Env.WORKER_STATUS_FILE}}']\n`,
    )
    const session = await openCli(scratchDir(), [], { HOME: home, TZ: 'UTC' })
    await session.waitForText('│ 09:00 ● event 01 ')
    expect(await session.text()).not.toContain('event 40')

    await session.press('tab')
    for (let presses = 0; presses < 20 && !(await session.text()).includes('event 40'); presses++) {
      await session.press('down')
    }

    await session.waitForText('│ 09:39 ● event 40 ')
    const screen = await session.text()
    expect(screen).not.toContain('not loaded  PID -')
    expect(screen).toContain('▌○ 01')
    expect(screen).toContain('╭ com.example.worker.01 ─')
  })

  it('? shows all keys, and q ends svcview while the help shows', async () => {
    const session = await openCli(scratchDir(), [], { HOME: homeWithPlists(['com.example.worker']) })
    await session.waitForText('com.example.worker')
    expect(await session.text()).toContain('? help')

    await session.type('?')

    await session.waitForText('select a service')
    expect(await session.text()).toContain('quit')

    await session.press('q')

    await session.waitForText('EXIT:0')
  })

  it('counts the services in each state in the header', async () => {
    const home = homeWithPlists([
      'com.example.a-running',
      'com.example.b-running',
      'com.example.c-loaded',
      'com.example.d-not-loaded',
      'com.example.e-not-loaded',
      'com.example.f-not-loaded',
    ])
    setLaunchctlList(home, [
      { name: 'com.example.a-running', pid: 4122 },
      { name: 'com.example.b-running', pid: 4123 },
      { name: 'com.example.c-loaded' },
    ])
    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('com.example.f-not-loaded')

    expect(await session.text()).toContain('● 2 running   ◐ 1 loaded   ○ 3 not loaded')
  })

  it('when the header line of launchctl list is not PID, Status, Label, shows the error and no rows, until the output is valid again', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlOutput(home, 'PID\tStatus\tName\n-\t0\tcom.example.worker\n')
    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('printed the header line')
    expect(await session.text()).not.toContain('com.example.worker')

    setLaunchctlList(home, [{ name: 'com.example.worker' }])

    await session.waitForText('com.example.worker', { timeout: 5000 })
    expect(await session.text()).not.toContain('printed the header line')
  })
})
