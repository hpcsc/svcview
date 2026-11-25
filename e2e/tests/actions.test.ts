import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { homeWithPlists, openCli, scratchDir, writeConfig } from '../testUtils'

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
})
