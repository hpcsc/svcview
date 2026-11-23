import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { homeWithPlists, openCli, scratchDir } from '../testUtils'

describe('the screen', () => {
  it('in a terminal, with no command, shows the domain and the services in LaunchAgents, sorted by name', async () => {
    const home = homeWithPlists(['homebrew.mxcl.redis', 'com.example.worker', 'com.apple.a'])

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('homebrew.mxcl.redis')
    const screen = await session.text()
    expect(screen).toContain('svcview')
    expect(screen).toContain(`gui/${process.getuid?.()}`)
    const rows = ['com.apple.a', 'com.example.worker', 'homebrew.mxcl.redis'].map((name) => screen.indexOf(name))
    expect(rows).not.toContain(-1)
    expect(rows).toEqual([...rows].sort((a, b) => a - b))
  })

  it('q ends svcview with the exit code 0', async () => {
    const session = await openCli(scratchDir(), [], { HOME: homeWithPlists(['com.example.worker']) })
    await session.waitForText('com.example.worker')

    await session.press('q')

    await session.waitForText('EXIT:0')
  })

  it('with stdin not a terminal, prints the help and does not open the screen', async () => {
    const session = await openCli(scratchDir(), ['<', '/dev/null'], { HOME: homeWithPlists(['com.example.worker']) })

    await session.waitForText('EXIT:0')

    const screen = await session.text()
    expect(screen).toContain('print the tag svcview was built from')
    expect(screen).not.toContain('com.example.worker')
  })

  it('with stdout not a terminal, prints the help and does not open the screen', async () => {
    const dir = scratchDir()
    const session = await openCli(dir, ['>', 'output.txt'], { HOME: homeWithPlists(['com.example.worker']) })

    await session.waitForText('EXIT:0')

    expect(readFileSync(join(dir, 'output.txt'), 'utf8')).toContain('print the tag svcview was built from')
    expect(await session.text()).not.toContain('com.example.worker')
  })

  it('with no home folder, says so and exits with 1', async () => {
    const session = await openCli(scratchDir(), [], { HOME: '' })

    await session.waitForText('EXIT:1')

    expect(await session.text()).toContain('$HOME is not defined')
  })
})
