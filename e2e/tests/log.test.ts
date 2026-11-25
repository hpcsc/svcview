import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { describe, expect, it } from 'vitest'
import { addPlist, failLess, homeWithPlists, lessCalls, openCli, scratchDir, writeConfig } from '../testUtils'

function isRunning(pid: number): boolean {
  try {
    process.kill(pid, 0)
    return true
  } catch {
    return false
  }
}

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

  it('ctrl-c while less has the terminal reaches less and does not end svcview: after q, the screen shows again, and ↓ selects the next row', async () => {
    const home = homeWithPlists(['com.example.x'])
    addPlist(home, 'com.example.worker', {}, { stdoutPath: '/var/log/com.example.worker.out.log' })
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker')
    await session.press('o')
    await session.waitForText('the fake less shows: +F -- /var/log/com.example.worker.out.log')

    await session.press(['ctrl', 'c'])

    await session.waitForText('the fake less got SIGINT')
    await session.type('q')
    await session.press('enter')
    await session.waitForText('▌○ com.example.worker')
    await session.press('down')
    await session.waitForText('▌○ com.example.x')
  })

  it('o on a service with only a StandardErrorPath gives the terminal to less +F with the StandardErrorPath', async () => {
    const home = homeWithPlists([])
    addPlist(home, 'com.example.worker', {}, { stderrPath: '/var/log/com.example.worker.err.log' })
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker')

    await session.press('o')

    await session.waitForText('the fake less shows: +F -- /var/log/com.example.worker.err.log')
    expect(lessCalls(home)).toEqual(['+F -- /var/log/com.example.worker.err.log'])
  })

  it('o on a service with no log path shows on the line above the help line that the service has no log path, and runs no less', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker')

    await session.press('o')

    await session.waitForText('com.example.worker has no log path in its plist')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('? help'))
    expect(lines[help - 1]).toMatch(/^ com\.example\.worker has no log path in its plist\s*$/)
    await sleep(500)
    expect(lessCalls(home)).toEqual([])
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

  it('SIGTERM while less has the terminal stops the status runs at once, a ctrl-c then still reaches less, and svcview ends when less ends', async () => {
    const home = homeWithPlists([])
    addPlist(home, 'com.example.worker.01', {}, { stdoutPath: '/var/log/com.example.worker.01.out.log' })
    const marker = join(scratchDir(), 'written')
    writeConfig(
      join(home, '.config', 'svcview'),
      `groups:\n  - name: Workers\n    match: '^com\\.example\\.worker\\.'\n    status:\n      command: [sh, -c, 'echo $PPID > "$0.svcview.new"; mv "$0.svcview.new" "$0.svcview"; (sleep 2; touch "$0") & sleep 30', '${marker}']\n      timeout: 10s\n`,
    )
    const session = await openCli(scratchDir(), [], { HOME: home })
    await expect.poll(() => existsSync(`${marker}.svcview`)).toBe(true)
    const svcview = Number(readFileSync(`${marker}.svcview`, 'utf8').trim())
    await session.press('o')
    await session.waitForText('the fake less shows: +F -- /var/log/com.example.worker.01.out.log')

    process.kill(svcview, 'SIGTERM')
    await sleep(3000)
    expect(existsSync(marker)).toBe(false)
    expect(isRunning(svcview)).toBe(true)
    await session.press(['ctrl', 'c'])
    await session.waitForText('the fake less got SIGINT')
    await sleep(500)
    expect(isRunning(svcview)).toBe(true)
    await session.type('q')
    await session.press('enter')

    await expect.poll(() => isRunning(svcview)).toBe(false)
  })
})
