import { describe, expect, it } from 'vitest'
import { addPlist, failLess, homeWithPlists, lessCalls, openCli, scratchDir } from '../testUtils'

describe('the log of a service', () => {
  it('↓ and o on a service with a StandardOutPath give the terminal to less +F with the path, and after q the screen shows again with the same service selected', async () => {
    const home = homeWithPlists(['com.example.a'])
    addPlist(home, 'com.example.worker', {}, { stdoutPath: '/var/log/com.example.worker.out.log' })
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.a')
    await session.press('down')
    await session.waitForText('▌○ com.example.worker')

    await session.press('o')

    await session.waitForText('the fake less shows: +F -- /var/log/com.example.worker.out.log')
    expect(await session.text()).not.toContain('? help')
    expect(lessCalls(home)).toEqual(['+F -- /var/log/com.example.worker.out.log'])
    await session.type('q')
    await session.press('enter')
    await session.waitForText('▌○ com.example.worker')
    expect(await session.text()).toContain('○ com.example.a')
  })

  it('when less exits with 1, the line above the help line names the command and the code', async () => {
    const home = homeWithPlists([])
    addPlist(home, 'com.example.worker', {}, { stdoutPath: '/var/log/com.example.worker.out.log' })
    failLess(home, 1)
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker')

    await session.press('o')

    await session.waitForText('exited with code 1')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('? help'))
    expect(lines[help - 1]).toMatch(/^ less \+F -- \/var\/log\/com\.example\.worker\.out\.log exited with code 1\s*$/)
    expect(lessCalls(home)).toEqual(['+F -- /var/log/com.example.worker.out.log'])
  })
})
