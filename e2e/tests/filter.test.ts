import { setTimeout as sleep } from 'node:timers/promises'
import { describe, expect, it } from 'vitest'
import { homeWithPlists, launchctlCalls, openCli, scratchDir, setLaunchctlList } from '../testUtils'

describe('the filter of the list', () => {
  it('on a running service, / and the text proxy show the filter line /proxy, the screen stays open, launchctl gets no call, and esc then hides the filter line', async () => {
    const home = homeWithPlists(['com.example.proxy'])
    setLaunchctlList(home, [{ name: 'com.example.proxy', pid: 4122 }])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌● com.example.proxy')

    await session.type('/')
    await session.type('proxy')

    await session.waitForText(' /proxy')
    await sleep(500)
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('esc clear the filter'))
    expect(lines[help - 1]).toMatch(/^ \/proxy\s*$/)
    expect(lines.join('\n')).not.toContain('EXIT:')
    expect(launchctlCalls(home)).toEqual([])

    await session.press('esc')

    await expect.poll(async () => (await session.text()).includes('/proxy')).toBe(false)
    expect(await session.text()).toContain('? help')
  })
})
