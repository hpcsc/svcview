import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { failLaunchctl, homeWithPlists, launchctlCalls, openCli, scratchDir, setLaunchctlList } from '../testUtils'

const uid = process.getuid?.()

describe('start a service', () => {
  it('s on a not loaded service runs launchctl bootstrap with the gui domain and the plist of the service', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker')

    await session.press('s')

    await expect
      .poll(() => launchctlCalls(home))
      .toEqual([`bootstrap gui/${uid} ${join(home, 'Library', 'LaunchAgents', 'com.example.worker.plist')}`])
  })

  it('s on a loaded service runs launchctl kickstart with the service in the gui domain', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker' }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌◐ com.example.worker')

    await session.press('s')

    await expect.poll(() => launchctlCalls(home)).toEqual([`kickstart gui/${uid}/com.example.worker`])
  })

  it('s on a running service runs no launchctl command', async () => {
    const home = homeWithPlists(['com.example.a-running', 'com.example.b-not-loaded'])
    setLaunchctlList(home, [{ name: 'com.example.a-running', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌● com.example.a-running')

    await session.press('s')
    await session.press('down')
    await session.waitForText('▌○ com.example.b-not-loaded')
    await session.press('s')

    await expect
      .poll(() => launchctlCalls(home))
      .toEqual([`bootstrap gui/${uid} ${join(home, 'Library', 'LaunchAgents', 'com.example.b-not-loaded.plist')}`])
  })

  it('after s, the row shows the new state at the next refresh, with no key press', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker' }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌◐ com.example.worker')

    await session.press('s')
    await expect.poll(() => launchctlCalls(home)).toEqual([`kickstart gui/${uid}/com.example.worker`])
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4122 }])

    await session.waitForText('▌● com.example.worker', { timeout: 5000 })
  })

  it('when launchctl fails, the line above the help line shows what it printed', async () => {
    const home = homeWithPlists(['com.example.worker'])
    failLaunchctl(home, 'Bootstrap failed: 5: Input/output error\n')
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker')

    await session.press('s')

    await session.waitForText('Bootstrap failed: 5: Input/output error')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.startsWith(' s start · x stop · ? help'))
    expect(lines[help - 1]).toMatch(/^ Bootstrap failed: 5: Input\/output error\s*$/)
  })
})

describe('stop a service', () => {
  it('x and y on a running service run launchctl bootout with the service in the gui domain', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌● com.example.worker')

    await session.press('x')
    await session.waitForText('Stop com.example.worker? (y/N)')
    await session.press('y')

    await expect.poll(() => launchctlCalls(home)).toEqual([`bootout gui/${uid}/com.example.worker`])
  })

  it('x and n run no launchctl command', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌● com.example.worker')

    await session.press('x')
    await session.waitForText('Stop com.example.worker? (y/N)')
    await session.press('n')
    await expect.poll(async () => (await session.text()).includes('Stop com.example.worker? (y/N)')).toBe(false)
    await session.press('x')
    await session.waitForText('Stop com.example.worker? (y/N)')
    await session.press('y')

    await expect.poll(() => launchctlCalls(home)).toEqual([`bootout gui/${uid}/com.example.worker`])
  })

  it('x on a not loaded service shows no confirmation and runs no launchctl command', async () => {
    const home = homeWithPlists(['com.example.a-not-loaded', 'com.example.b-running'])
    setLaunchctlList(home, [{ name: 'com.example.b-running', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.a-not-loaded')

    await session.press('x')
    await session.press('down')
    await session.waitForText('▌● com.example.b-running')
    expect(await session.text()).not.toContain('Stop com.example.a-not-loaded?')
    await session.press('x')
    await session.waitForText('Stop com.example.b-running? (y/N)')
    await session.press('y')

    await expect.poll(() => launchctlCalls(home)).toEqual([`bootout gui/${uid}/com.example.b-running`])
  })

  it('after x, y and a launchctl list without the service, the row shows ○ at the next refresh and at the refresh after it, and svcview starts nothing', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌● com.example.worker')

    await session.press('x')
    await session.waitForText('Stop com.example.worker? (y/N)')
    await session.press('y')
    await expect.poll(() => launchctlCalls(home)).toEqual([`bootout gui/${uid}/com.example.worker`])
    setLaunchctlList(home, [])

    await session.waitForText('▌○ com.example.worker', { timeout: 5000 })
    await new Promise((done) => setTimeout(done, 2500))
    expect(await session.text()).toContain('▌○ com.example.worker')
    expect(launchctlCalls(home)).toEqual([`bootout gui/${uid}/com.example.worker`])
  })
})
