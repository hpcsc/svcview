import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, it } from 'vitest'
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
})
