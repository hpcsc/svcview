import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { describe, expect, it } from 'vitest'
import {
  homeWithPlists,
  launchctlCalls,
  lessCalls,
  listLines,
  openCli,
  scratchDir,
  setLaunchctlList,
  writeConfig,
} from '../testUtils'

function homeWithServicesWithAndWithoutPlist(): string {
  const home = homeWithPlists(['com.example.worker', 'com.example.not-loaded'])
  setLaunchctlList(home, [
    { name: 'com.example.worker', pid: 4122 },
    { name: 'com.apple.a', pid: 4123, status: -9 },
    { name: 'com.apple.b' },
  ])
  return home
}

describe('svcview --all', () => {
  it('shows a row for each name that launchctl list prints, also each name that no plist has, and the header counts each of these services', async () => {
    const session = await openCli(scratchDir(), ['--all'], { HOME: homeWithServicesWithAndWithoutPlist() })

    await session.waitForText('com.example.worker')
    const screen = await session.text()
    expect(screen).toContain('com.apple.a')
    expect(screen).toContain('com.apple.b')
    expect(screen).toContain('com.example.not-loaded')
    expect(screen).toContain('● 2 running   ◐ 1 loaded   ○ 1 not loaded')
  })

  it('with no flag, on the same plists and the same launchctl list, shows only the services that have a plist, and the header counts only them', async () => {
    const session = await openCli(scratchDir(), [], { HOME: homeWithServicesWithAndWithoutPlist() })

    await session.waitForText('com.example.worker')
    const screen = await session.text()
    expect(screen).toContain('com.example.not-loaded')
    expect(screen).not.toContain('com.apple.a')
    expect(screen).not.toContain('com.apple.b')
    expect(screen).toContain('● 1 running   ◐ 0 loaded   ○ 1 not loaded')
  })

  it('shows · all after the domain in the header', async () => {
    const session = await openCli(scratchDir(), ['--all'], { HOME: homeWithServicesWithAndWithoutPlist() })

    await session.waitForText('com.example.worker')
    expect(await session.text()).toContain(`gui/${process.getuid?.()} · all`)
  })

  it('with no flag, on the same plists and the same launchctl list, shows the domain with no · all in the header', async () => {
    const session = await openCli(scratchDir(), [], { HOME: homeWithServicesWithAndWithoutPlist() })

    await session.waitForText('com.example.worker')
    const screen = await session.text()
    expect(screen).toContain(`gui/${process.getuid?.()}`)
    expect(screen).not.toContain('· all')
  })

  it('shows the state, the PID, the uptime and the last exit of a running service with no plist in the detail pane', async () => {
    const home = homeWithPlists([])
    setLaunchctlList(home, [{ name: 'com.apple.a', pid: 4123, status: -9 }])

    const session = await openCli(scratchDir(), ['--all'], { HOME: home })

    await session.waitForText('PID 4123')
    const screen = await session.text()
    expect(screen).toContain('╭ com.apple.a ─')
    expect(screen).toContain('● running     PID 4123')
    expect(screen).toContain('up 3h12m      last exit SIGKILL')
  })

  it('shows a service with no plist that a group matches under the line of the group with its short name, and a service with no plist that no group matches under Other', async () => {
    const home = homeWithPlists(['com.example.worker.01'])
    setLaunchctlList(home, [{ name: 'com.example.worker.02' }, { name: 'com.apple.a' }])
    writeConfig(join(home, '.config', 'svcview'), 'groups:\n  - name: Workers\n    match: ^com\\.example\\.worker\\.\n')

    const session = await openCli(scratchDir(), ['--all'], { HOME: home })

    await session.waitForText('com.apple.a')
    expect(listLines(await session.text()).slice(0, 6)).toEqual([
      'Workers',
      '▌○ 01',
      '◐ 02',
      '',
      'Other',
      '◐ com.apple.a',
    ])
  })

  it('a group whose status command uses a variable of the plist shows the plist has no and the name of the variable in the row of a service with no plist', async () => {
    const home = homeWithPlists([])
    setLaunchctlList(home, [{ name: 'com.example.worker.01', pid: 4122 }])
    writeConfig(
      join(home, '.config', 'svcview'),
      "groups:\n  - name: Workers\n    match: '^com\\.example\\.worker\\.'\n    status:\n      command: [cat, '{{.Env.WORKER_STATUS_FILE}}']\n",
    )

    const session = await openCli(scratchDir(), ['--all'], { HOME: home })

    await session.waitForText('01  the plist has no WORKER_STATUS_FILE')
  })

  it('the key of an action whose command uses a variable of the plist, on a service with no plist, shows the plist has no and the name of the variable on the line above the help line, and runs no less', async () => {
    const home = homeWithPlists([])
    setLaunchctlList(home, [{ name: 'com.example.worker.01' }])
    writeConfig(
      join(home, '.config', 'svcview'),
      "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - key: L\n        name: worker log\n        command: [less, +F, '{{.Env.WORKER_LOG}}']\n",
    )
    const session = await openCli(scratchDir(), ['--all'], { HOME: home })
    await session.waitForText('▌◐ com.example.worker.01')

    await session.type('L')

    await session.waitForText('the plist has no WORKER_LOG')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('? help'))
    expect(lines[help - 1]).toMatch(/^ the plist has no WORKER_LOG\s*$/)
    await sleep(500)
    expect(lessCalls(home)).toEqual([])
  })

  it('o on a service with no plist shows on the line above the help line that the service has no log path, and runs no less', async () => {
    const home = homeWithPlists([])
    setLaunchctlList(home, [{ name: 'com.apple.a' }])
    const session = await openCli(scratchDir(), ['--all'], { HOME: home })
    await session.waitForText('▌◐ com.apple.a')

    await session.press('o')

    await session.waitForText('com.apple.a has no log path in its plist')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('? help'))
    expect(lines[help - 1]).toMatch(/^ com\.apple\.a has no log path in its plist\s*$/)
    await sleep(500)
    expect(lessCalls(home)).toEqual([])
  })
})

describe('the key a', () => {
  it('shows each service that launchctl list prints and · all after the domain, a again shows only the services that have a plist and no · all, and launchctl gets no call other than list', async () => {
    const home = homeWithServicesWithAndWithoutPlist()
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('com.example.worker')
    expect(await session.text()).not.toContain('com.apple.a')

    await session.type('a')

    await session.waitForText('com.apple.a', { timeout: 5000 })
    expect(await session.text()).toContain(`gui/${process.getuid?.()} · all`)

    await session.type('a')

    await expect.poll(async () => (await session.text()).includes('com.apple.a'), { timeout: 5000 }).toBe(false)
    expect(await session.text()).not.toContain('· all')
    expect(launchctlCalls(home)).toEqual([])
  })
})
