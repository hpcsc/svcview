import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { describe, expect, it } from 'vitest'
import { homeWithPlists, launchctlCalls, openCli, scratchDir, setLaunchctlList, writeConfig } from '../testUtils'

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

  it('with the group Workers in the config, / and WORKER show the rows of Workers and no line Other, and esc shows each service again', async () => {
    const home = homeWithPlists(['com.example.worker.01', 'com.example.worker.02', 'com.apple.a'])
    writeConfig(join(home, '.config', 'svcview'), 'groups:\n  - name: Workers\n    match: worker\n')
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker.01')
    expect(await session.text()).toContain('Other')

    await session.type('/')
    await session.type('WORKER')

    await session.waitForText(' /WORKER')
    const filtered = await session.text()
    expect(filtered).toContain('Workers')
    expect(filtered).toContain('▌○ com.example.worker.01')
    expect(filtered).toContain('○ com.example.worker.02')
    expect(filtered).not.toContain('Other')
    expect(filtered).not.toContain('com.apple.a')

    await session.press('esc')

    await session.waitForText('○ com.apple.a')
    expect(await session.text()).toContain('Other')
  })
})
