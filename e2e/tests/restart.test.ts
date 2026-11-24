import { describe, expect, it } from 'vitest'
import { homeWithPlists, launchctlCalls, openCli, scratchDir, setLaunchctlList, setPsElapsed } from '../testUtils'

const uid = process.getuid?.()

describe('restart a service', () => {
  it('r and y on a running service run launchctl kickstart -k with the service in the gui domain', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌● com.example.worker')

    await session.press('r')
    await session.waitForText('Restart com.example.worker? (y/N)')
    await session.press('y')

    await expect.poll(() => launchctlCalls(home)).toEqual([`kickstart -k gui/${uid}/com.example.worker`])
  })

  it('r and y on a loaded service run launchctl kickstart -k with the service in the gui domain', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker' }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌◐ com.example.worker')

    await session.press('r')
    await session.waitForText('Restart com.example.worker? (y/N)')
    await session.press('y')

    await expect.poll(() => launchctlCalls(home)).toEqual([`kickstart -k gui/${uid}/com.example.worker`])
  })

  it('r and n run no launchctl command', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌● com.example.worker')

    await session.press('r')
    await session.waitForText('Restart com.example.worker? (y/N)')
    await session.press('n')
    await expect.poll(async () => (await session.text()).includes('Restart com.example.worker? (y/N)')).toBe(false)
    await session.press('r')
    await session.waitForText('Restart com.example.worker? (y/N)')
    await session.press('y')

    await expect.poll(() => launchctlCalls(home)).toEqual([`kickstart -k gui/${uid}/com.example.worker`])
  })

  it('r on a not loaded service shows on the line above the help line that the service is not loaded, and runs no launchctl command', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker')

    await session.press('r')

    await session.waitForText('com.example.worker is not loaded, so svcview cannot restart it')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('? help'))
    expect(lines[help - 1]).toMatch(/^ com\.example\.worker is not loaded, so svcview cannot restart it\s*$/)
    await session.press('y')
    await new Promise((done) => setTimeout(done, 500))
    expect(launchctlCalls(home)).toEqual([])
  })

  it('after r, y, a launchctl list with a new PID and a new elapsed time of ps, the detail pane shows the new PID and the new uptime at the next refresh, with no key press', async () => {
    const home = homeWithPlists(['com.example.worker'])
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('PID 4122')
    expect(await session.text()).toContain('up 3h12m')

    await session.press('r')
    await session.waitForText('Restart com.example.worker? (y/N)')
    await session.press('y')
    await expect.poll(() => launchctlCalls(home)).toEqual([`kickstart -k gui/${uid}/com.example.worker`])
    setPsElapsed(home, '00:05')
    setLaunchctlList(home, [{ name: 'com.example.worker', pid: 4123 }])

    await session.waitForText('PID 4123', { timeout: 5000 })
    const text = await session.text()
    expect(text).toContain('up 5s')
    expect(text).not.toContain('up 3h12m')
  })
})
