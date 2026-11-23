import { existsSync, renameSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { describe, expect, it } from 'vitest'
import { addPlist, homeWithPlists, openCli, scratchDir, writeConfig } from '../testUtils'

const workersWithStatusCommand = (command: string): string =>
  `groups:\n  - name: Workers\n    match: '^com\\.example\\.worker\\.'\n    status:\n      command: ${command}\n`

describe('the status command', () => {
  it('a group whose status command is cat of the status file that the plist names shows the summary of the file in the row of the service', async () => {
    const home = homeWithPlists([])
    const statusFile = join(scratchDir(), 'status.json')
    writeFileSync(statusFile, '{"version":1,"summary":"busy · ENG-692","level":"warn"}\n')
    addPlist(home, 'com.example.worker.01', { WORKER_STATUS_FILE: statusFile })
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[cat, '{{.Env.WORKER_STATUS_FILE}}']`))

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('01  busy · ENG-692')
  })

  it('a group whose status command uses a variable that the plist does not have shows the plist has no and the name of the variable in the row of the service', async () => {
    const home = homeWithPlists([])
    addPlist(home, 'com.example.worker.01', { WORKER_STATUS_FILE: '/tmp/status.json' })
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[cat, '{{.Env.MISSING}}']`))

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('01  the plist has no MISSING')
  })

  it('when the status file changes, the row shows the new summary with no key press', async () => {
    const home = homeWithPlists([])
    const statusFile = join(scratchDir(), 'status.json')
    writeFileSync(statusFile, '{"version":1,"summary":"idle"}\n')
    addPlist(home, 'com.example.worker.01', { WORKER_STATUS_FILE: statusFile })
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[cat, '{{.Env.WORKER_STATUS_FILE}}']`))

    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('01  idle')
    writeFileSync(`${statusFile}.new`, '{"version":1,"summary":"busy"}\n')
    renameSync(`${statusFile}.new`, statusFile)

    await session.waitForText('01  busy', { timeout: 5000 })
  })

  it('a status command that exits with 3 shows status error in the row, and the exit code and the lines of stderr in the detail pane', async () => {
    const home = homeWithPlists(['com.example.worker.01'])
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[sh, -c, 'echo broken >&2; exit 3']`))

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('01  status error')
    await session.waitForText('exit code 3')
    await session.waitForText('│ broken ')
  })

  it('a status command that writes a file after 2 s, with a timeout of 10 s, does not write it when svcview quits after the run starts', async () => {
    const home = homeWithPlists(['com.example.worker.01'])
    const marker = join(scratchDir(), 'written')
    writeConfig(
      join(home, '.config', 'svcview'),
      `groups:\n  - name: Workers\n    match: '^com\\.example\\.worker\\.'\n    status:\n      command: [sh, -c, 'touch "$0.started"; sleep 2; touch "$0"', '${marker}']\n      timeout: 10s\n`,
    )

    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('com.example.worker.01')
    await expect.poll(() => existsSync(`${marker}.started`)).toBe(true)
    await session.press('q')
    await session.waitForText('EXIT:0')
    await sleep(3000)

    expect(existsSync(marker)).toBe(false)
  })
})
