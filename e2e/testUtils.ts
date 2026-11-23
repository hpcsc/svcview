import { spawn } from 'node:child_process'
import { chmodSync, mkdirSync, mkdtempSync, renameSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { launchTerminal, type Session } from 'tuistory'
import { onTestFinished } from 'vitest'

const fakes = fileURLToPath(new URL('fakes', import.meta.url))

export function getExecutablePath(): string {
  const executablePath = process.env.EXECUTABLE
  if (!executablePath) {
    throw new Error('EXECUTABLE environment variable is required. Set it to the path of the svcview binary.')
  }
  return resolve(executablePath)
}

export function buildTag(): string {
  const tag = process.env.BUILD_TAG
  if (!tag) {
    throw new Error('BUILD_TAG environment variable is required. Set it to the tag the svcview binary was built with.')
  }
  return tag
}

export function scratchDir(): string {
  const dir = mkdtempSync(join(tmpdir(), 'svcview-e2e-'))
  onTestFinished(() => rmSync(dir, { recursive: true, force: true }))
  return dir
}

export function addPlist(home: string, name: string): void {
  const folder = join(home, 'Library', 'LaunchAgents')
  mkdirSync(folder, { recursive: true })
  writeFileSync(
    join(folder, `${name}.plist`),
    `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>${name}</string>
</dict>
</plist>
`,
  )
}

export interface ListedService {
  name: string
  pid?: number
  status?: number
}

export function setLaunchctlList(home: string, services: ListedService[]): void {
  const rows = services.map((service) => `${service.pid ?? '-'}\t${service.status ?? 0}\t${service.name}\n`)
  setLaunchctlOutput(home, `PID\tStatus\tLabel\n${rows.join('')}`)
}

export function setLaunchctlOutput(home: string, output: string): void {
  const path = join(home, 'launchctl-list.txt')
  writeFileSync(`${path}.new`, output)
  // one rename, so the fake never prints half of the file while the screen is open
  renameSync(`${path}.new`, path)
}

export function homeWithPlists(names: string[]): string {
  const home = scratchDir()
  mkdirSync(join(home, 'Library', 'LaunchAgents'), { recursive: true })
  for (const name of names) addPlist(home, name)
  setLaunchctlList(home, [])
  return home
}

export function writeConfig(configFolder: string, content: string, mode = 0o644): string {
  mkdirSync(configFolder, { recursive: true })
  const path = join(configFolder, 'config.yaml')
  writeFileSync(path, content)
  chmodSync(path, mode)
  return path
}

export interface Result {
  stdout: string
  stderr: string
  status: number | null
}

// runCli does not block, so a fake server in this process can still answer the
// requests the CLI makes. With no input, stdin stays open, so a CLI that reads
// stdin does not end.
export function runCli(
  cwd: string,
  args: string[] = [],
  env: Record<string, string> = {},
  executable = getExecutablePath(),
  input?: string,
): Promise<Result> {
  const { XDG_CONFIG_HOME: _, ...inherited } = process.env
  return new Promise((done, fail) => {
    const child = spawn(executable, args, { cwd, env: { ...inherited, ...env } })
    if (input !== undefined) child.stdin.end(input)
    let stdout = ''
    let stderr = ''
    child.stdout.on('data', (chunk) => (stdout += chunk))
    child.stderr.on('data', (chunk) => (stderr += chunk))
    child.on('error', fail)
    child.on('close', (status) => done({ stdout, stderr, status }))
  })
}

// openCli starts the CLI in a pseudo-terminal. When the CLI stops, the shell
// writes its exit status on the screen, so a test can wait for EXIT:0.
export async function openCli(cwd: string, args: string[] = [], env: Record<string, string> = {}): Promise<Session> {
  const assignments = Object.entries({ PATH: `${fakes}:${process.env.PATH}`, ...env })
    .map(([name, value]) => `${name}='${value.replaceAll("'", `'\\''`)}'`)
    .join(' ')
  const session = await launchTerminal({
    command: 'sh',
    // The emulator starts in new line mode, where a line feed also goes back to
    // column 1. A real terminal does not, and a full screen program that moves
    // the cursor down with a line feed then draws in the wrong column, so
    // \e[20l turns the mode off before the CLI starts.
    args: [
      '-c',
      `printf '\\033[20l'; unset XDG_CONFIG_HOME; ${assignments} "${getExecutablePath()}" ${args.join(' ')}; echo "EXIT:$?"`,
    ],
    cwd,
    cols: 160,
    rows: 40,
  })
  onTestFinished(() => session.close())
  return session
}
