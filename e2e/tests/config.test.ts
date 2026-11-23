import { join } from 'node:path'
import { describe, expect, it, onTestFinished, vi } from 'vitest'
import { buildTag, homeWithPlists, openCli, runCli, scratchDir, writeConfig } from '../testUtils'

describe('the config file', () => {
  it('an unknown setting in $XDG_CONFIG_HOME/svcview/config.yaml stops svcview with the exit code 1 and an error that names the file, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const xdgConfigHome = scratchDir()
    const path = writeConfig(join(xdgConfigHome, 'svcview'), 'colour: blue\n')

    const session = await openCli(scratchDir(), [], { HOME: home, XDG_CONFIG_HOME: xdgConfigHome })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('"colour"')
    expect(printed).not.toContain('com.example.worker')
  })

  it('with no XDG_CONFIG_HOME, an unknown setting in $HOME/.config/svcview/config.yaml stops svcview with the exit code 1 and an error that names the file, also when the test run sets XDG_CONFIG_HOME', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const path = writeConfig(join(home, '.config', 'svcview'), 'colour: blue\n')
    const xdgConfigHomeOfTheRun = scratchDir()
    writeConfig(join(xdgConfigHomeOfTheRun, 'svcview'), 'refresh: 5s\n')
    vi.stubEnv('XDG_CONFIG_HOME', xdgConfigHomeOfTheRun)
    onTestFinished(() => vi.unstubAllEnvs())

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('"colour"')
    expect(printed).not.toContain('com.example.worker')
  })

  it('a config.yaml that all users can write stops svcview with the exit code 1 and an error that names the file, before the screen opens', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const path = writeConfig(join(home, '.config', 'svcview'), 'refresh: 5s\n', 0o666)

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('EXIT:1')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain(path)
    expect(printed).toContain('another user can write')
    expect(printed).not.toContain('com.example.worker')
  })

  it('a valid config.yaml opens the screen', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfig(join(home, '.config', 'svcview'), 'refresh: 5s\n')

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('com.example.worker')
  })

  it('svcview version does not read the config file', async () => {
    const home = homeWithPlists([])
    writeConfig(join(home, '.config', 'svcview'), 'colour: blue\n')

    const result = await runCli(scratchDir(), ['version'], { HOME: home })

    expect(result.status).toBe(0)
    expect(result.stdout.trim()).toBe(buildTag())
  })

  it('the help that svcview prints without a terminal does not read the config file', async () => {
    const home = homeWithPlists([])
    writeConfig(join(home, '.config', 'svcview'), 'colour: blue\n')

    const result = await runCli(scratchDir(), [], { HOME: home })

    expect(result.status).toBe(0)
    expect(result.stdout).toContain('print the tag svcview was built from')
  })
})
