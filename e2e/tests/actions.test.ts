import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { describe, expect, it } from 'vitest'
import { addPlist, homeWithPlists, lessCalls, openCli, scratchDir, writeConfig } from '../testUtils'

const workerLogAction =
  "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - key: L\n        name: worker log\n        command: [less, +F, '{{.Env.WORKER_LOG}}']\n"

describe('the actions of a group', () => {
  it('an action with the key s, a key of svcview, stops svcview with the exit code 1 and an error that names the file and the group, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker.01'])
    const path = writeConfig(
      join(home, '.config', 'svcview'),
      "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - key: s\n        name: worker log\n        command: [less, +F, '{{.Env.WORKER_LOG}}']\n",
    )

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('action 1 of group "Workers"')
    expect(printed).toContain('key of svcview')
    expect(printed).not.toContain('com.example.worker.01')
  })

  it('a group with two actions with the key L stops svcview with the exit code 1 and an error that names the file and the group, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker.01'])
    const path = writeConfig(
      join(home, '.config', 'svcview'),
      "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - key: L\n        name: worker log\n        command: [less, +F, '{{.Env.WORKER_LOG}}']\n      - key: L\n        name: worker errors\n        command: [less, +F, '{{.StderrPath}}']\n",
    )

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('action 2 of group "Workers"')
    expect(printed).toContain('"L"')
    expect(printed).not.toContain('com.example.worker.01')
  })

  it('the help line shows L worker log before / filter and a all while a service of the group is selected, and no worker log after ↓ selects a service of Other', async () => {
    const home = homeWithPlists(['com.example.worker.01', 'com.example.other'])
    writeConfig(
      join(home, '.config', 'svcview'),
      "groups:\n  - name: Workers\n    match: worker\n    actions:\n      - key: L\n        name: worker log\n        command: [less, +F, '{{.Env.WORKER_LOG}}']\n",
    )
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker.01')
    const helpLine = async () => (await session.text()).split('\n').find((line) => line.includes('? help'))

    expect(await helpLine()).toContain('o log · L worker log · / filter · a all · ? help')

    await session.press('down')
    await session.waitForText('▌○ com.example.other')
    expect(await helpLine()).toContain('o log · / filter · a all · ? help')
    expect(await helpLine()).not.toContain('worker log')
  })

  it('L gives the terminal to less with the WORKER_LOG of the plist of the selected service, and after q the screen shows again with the same service selected', async () => {
    const home = homeWithPlists([])
    addPlist(home, 'com.example.worker.01', { WORKER_LOG: '/Users/me/worker/01.log' })
    addPlist(home, 'com.example.worker.02', { WORKER_LOG: '/Users/me/worker/02.log' })
    writeConfig(join(home, '.config', 'svcview'), workerLogAction)
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker.01')
    await session.press('down')
    await session.waitForText('▌○ com.example.worker.02')

    await session.type('L')

    await session.waitForText('the fake less shows: +F /Users/me/worker/02.log')
    expect(lessCalls(home)).toEqual(['+F /Users/me/worker/02.log'])
    await session.type('q')
    await session.press('enter')
    await session.waitForText('▌○ com.example.worker.02')
  })

  it('ctrl-c while less has the terminal reaches less and does not end svcview: after q, the screen shows again, and ↑ selects the row above', async () => {
    const home = homeWithPlists([])
    addPlist(home, 'com.example.worker.01', { WORKER_LOG: '/Users/me/worker/01.log' })
    addPlist(home, 'com.example.worker.02', { WORKER_LOG: '/Users/me/worker/02.log' })
    writeConfig(join(home, '.config', 'svcview'), workerLogAction)
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker.01')
    await session.press('down')
    await session.waitForText('▌○ com.example.worker.02')
    await session.type('L')
    await session.waitForText('the fake less shows: +F /Users/me/worker/02.log')

    await session.press(['ctrl', 'c'])

    await session.waitForText('the fake less got SIGINT')
    await session.type('q')
    await session.press('enter')
    await session.waitForText('▌○ com.example.worker.02')
    await session.press('up')
    await session.waitForText('▌○ com.example.worker.01')
  })

  it('L on a service whose plist has no WORKER_LOG shows the plist has no WORKER_LOG on the line above the help line, and runs no less', async () => {
    const home = homeWithPlists(['com.example.worker.01'])
    writeConfig(join(home, '.config', 'svcview'), workerLogAction)
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker.01')

    await session.type('L')

    await session.waitForText('the plist has no WORKER_LOG')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('? help'))
    expect(lines[help - 1]).toMatch(/^ the plist has no WORKER_LOG\s*$/)
    await sleep(500)
    expect(lessCalls(home)).toEqual([])
  })

  it('with confirm: true, L shows the confirmation on the line above the help line, n hides it and runs no less, and L and then y give the terminal to less', async () => {
    const home = homeWithPlists([])
    addPlist(home, 'com.example.worker.01', { WORKER_LOG: '/Users/me/worker/01.log' })
    writeConfig(join(home, '.config', 'svcview'), `${workerLogAction}        confirm: true\n`)
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('▌○ com.example.worker.01')

    await session.type('L')

    await session.waitForText('Run worker log for com.example.worker.01? (y/N)')
    const lines = (await session.text()).split('\n')
    const help = lines.findIndex((line) => line.includes('? help'))
    expect(lines[help - 1]).toMatch(/^ Run worker log for com\.example\.worker\.01\? \(y\/N\)\s*$/)

    await session.type('n')
    await expect.poll(async () => await session.text()).not.toContain('(y/N)')
    await sleep(500)
    expect(lessCalls(home)).toEqual([])

    await session.type('L')
    await session.waitForText('(y/N)')
    await session.type('y')
    await session.waitForText('the fake less shows: +F /Users/me/worker/01.log')
    expect(lessCalls(home)).toEqual(['+F /Users/me/worker/01.log'])
  })
})
