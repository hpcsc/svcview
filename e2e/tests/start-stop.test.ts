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
