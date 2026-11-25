import { mkdirSync, renameSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { homeWithPlists, launchctlCalls, openCli, scratchDir, setLaunchctlList } from '../testUtils'

function plistText(name: string, properties: string): string {
  return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>${name}</string>
${properties}</dict>
</plist>
`
}

function writePlist(home: string, name: string, properties: string): string {
  const folder = join(home, 'Library', 'LaunchAgents')
  mkdirSync(folder, { recursive: true })
  const path = join(folder, `${name}.plist`)
  writeFileSync(path, plistText(name, properties))
  return path
}

const programAndPaths = `	<key>ProgramArguments</key>
	<array>
		<string>/usr/local/bin/worker</string>
		<string>--queue</string>
		<string>jobs</string>
	</array>
	<key>WorkingDirectory</key>
	<string>/Users/me/work</string>
	<key>StandardOutPath</key>
	<string>/Users/me/Library/Logs/worker.log</string>
	<key>EnvironmentVariables</key>
	<dict>
		<key>WORKER_TOKEN</key>
		<string>token-s3cret-value</string>
	</dict>
`

describe('the properties of the plist in the detail pane', () => {
  it('shows the program with each argument on a line of its own, the work folder, the stdout and the names of the environment, and no value of a variable, with no launchctl call other than list', async () => {
    const home = homeWithPlists([])
    writePlist(home, 'com.example.worker', programAndPaths)

    const session = await openCli(scratchDir(), [], { HOME: home })

    await session.waitForText('program       /usr/local/bin/worker')
    const screen = await session.text()
    const lines = screen.split('\n')
    const program = lines.findIndex((line) => line.includes('│ program       /usr/local/bin/worker '))
    expect(lines[program + 1]).toContain('│               --queue ')
    expect(lines[program + 2]).toContain('│               jobs ')
    expect(screen).toContain('│ work folder   /Users/me/work ')
    expect(screen).toContain('│ stdout        /Users/me/Library/Logs/worker.log ')
    expect(screen).toContain('│ environment   WORKER_TOKEN ')
    expect(screen).not.toContain('s3cret')
    expect(launchctlCalls(home)).toEqual([])
  })

  it('shows the program of a new plist that is renamed onto the plist while the screen is open, with no key press', async () => {
    const home = homeWithPlists([])
    const path = writePlist(home, 'com.example.worker', programAndPaths)
    const session = await openCli(scratchDir(), [], { HOME: home })
    await session.waitForText('program       /usr/local/bin/worker')

    writeFileSync(
      `${path}.new`,
      plistText('com.example.worker', '\t<key>ProgramArguments</key>\n\t<array>\n\t\t<string>/usr/local/bin/other</string>\n\t</array>\n'),
    )
    renameSync(`${path}.new`, path)

    await session.waitForText('program       /usr/local/bin/other', { timeout: 5000 })
  })

  it('with --all, shows no line Plist for a service with no plist', async () => {
    const home = homeWithPlists([])
    setLaunchctlList(home, [{ name: 'com.apple.a', pid: 4123 }])

    const session = await openCli(scratchDir(), ['--all'], { HOME: home })

    await session.waitForText('PID 4123')
    expect(await session.text()).not.toContain('Plist')
  })
})
