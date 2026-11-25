import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { describe, expect, it } from 'vitest'
import { addPlist, homeWithPlists, openCli, runCli, scratchDir, writeConfig } from '../testUtils'

const workersWithStatusCommand = (command: string, timeout = '1s'): string =>
  `groups:\n  - name: Workers\n    match: '^com\\.example\\.worker'\n    status:\n      command: ${command}\n      timeout: ${timeout}\n`

function homeWithStatusFile(content: string): { home: string; statusFile: string } {
  const home = homeWithPlists([])
  const statusFile = join(scratchDir(), 'status.json')
  writeFileSync(statusFile, content)
  addPlist(home, 'com.example.worker', { WORKER_STATUS_FILE: statusFile })
  return { home, statusFile }
}

describe('svcview status', () => {
  it('cat of a valid status file prints the template values with a variable of the plist, the command line, the output, and valid with the summary, and exits with 0', async () => {
    const { home, statusFile } = homeWithStatusFile('{"version":1,"summary":"busy"}\n')
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[cat, '{{.Env.WORKER_STATUS_FILE}}']`))

    const result = await runCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })

    expect(result.stdout).toContain('Template values\n  {{.Name}}')
    expect(result.stdout).toContain(`  {{.Env.WORKER_STATUS_FILE}}  ${statusFile}\n`)
    expect(result.stdout).toContain(`\nCommand line\n  cat ${statusFile}\n`)
    expect(result.stdout).toContain('\nOutput\n  {"version":1,"summary":"busy"}\n')
    expect(result.stdout).toContain('\nResult\n  valid\n')
    expect(result.stdout).toContain('  summary  busy\n')
    expect(result.status).toBe(0)
  })

  it('a command that exits with 3 prints status error, the exit code and the stderr, and exits with 1', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[sh, -c, 'echo broken >&2; exit 3']`))

    const result = await runCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })

    expect(result.stdout).toContain('\nResult\n  status error\n  exit code 3\n    broken\n')
    expect(result.status).toBe(1)
  })

  it('a command that runs longer than the timeout prints status timeout and the command line, and exits with 1', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[sleep, '30']`, '500ms'))
    const started = Date.now()

    const result = await runCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })

    expect(Date.now() - started).toBeLessThan(1500)
    expect(result.stdout).toContain('\nResult\n  status timeout\n  sleep 30\n')
    expect(result.status).toBe(1)
  })

  it('a command that prints text that is not JSON prints status invalid and the parse error, and exits with 1', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[printf, 'busy']`))

    const result = await runCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })

    expect(result.stdout).toContain('\nOutput\n  busy\n')
    expect(result.stdout).toContain('\nResult\n  status invalid\n  the output is not valid JSON')
    expect(result.status).toBe(1)
  })

  it('SIGINT during the run stops the program with each process that it started, prints no valid, and exits with 1', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const marker = join(scratchDir(), 'written')
    writeConfig(
      join(home, '.config', 'svcview'),
      workersWithStatusCommand(
        `[sh, -c, 'echo $PPID > "$0.svcview.new"; mv "$0.svcview.new" "$0.svcview"; (sleep 2; touch "$0") & sleep 30', '${marker}']`,
        '10s',
      ),
    )

    const running = runCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })
    await expect.poll(() => existsSync(`${marker}.svcview`)).toBe(true)
    process.kill(Number(readFileSync(`${marker}.svcview`, 'utf8').trim()), 'SIGINT')
    const result = await running
    await sleep(3000)

    expect(result.stdout).toContain('svcview got a signal and stopped the status command before the command ended')
    expect(result.stdout).not.toContain('valid')
    expect(result.status).toBe(1)
    expect(existsSync(marker)).toBe(false)
  })

  it('a variable that the plist does not have prints the plist has no and the variable, runs no program, and exits with 1', async () => {
    const home = homeWithPlists(['com.example.worker'])
    const marker = join(scratchDir(), 'ran')
    writeConfig(
      join(home, '.config', 'svcview'),
      workersWithStatusCommand(`[sh, -c, 'touch "$0"', '${marker}', '{{.Env.WORKER_STATUS_FILE}}']`),
    )

    const result = await runCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })

    expect(result.stdout).toContain('Command line\n  the plist has no WORKER_STATUS_FILE\n')
    expect(result.stdout.split('the plist has no WORKER_STATUS_FILE')).toHaveLength(2)
    expect(existsSync(marker)).toBe(false)
    expect(result.status).toBe(1)
  })

  it('a name that no service has exits with 1 and says that svcview knows only the services with a plist in ~/Library/LaunchAgents', async () => {
    const home = homeWithPlists(['com.example.worker'])
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[cat, status.json]`))

    const result = await runCli(scratchDir(), ['status', 'com.example.job'], { HOME: home })

    expect(result.stdout).toContain('"com.example.job"')
    expect(result.stdout).toContain('~/Library/LaunchAgents')
    expect(result.stdout).not.toContain('Template values')
    expect(result.status).toBe(1)
  })

  it('a service in the group Other exits with 1 and says that its group has no status command', async () => {
    const home = homeWithPlists(['com.example.job'])
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[cat, status.json]`))

    const result = await runCli(scratchDir(), ['status', 'com.example.job'], { HOME: home })

    expect(result.stdout).toContain('com.example.job')
    expect(result.stdout).toContain('"Other"')
    expect(result.stdout).toContain('has no status command')
    expect(result.stdout).not.toContain('Template values')
    expect(result.status).toBe(1)
  })

  it('a config.yaml that all users can write exits with 1 and an error that names the file, before it prints a step', async () => {
    const { home } = homeWithStatusFile('{"version":1,"summary":"busy"}\n')
    const path = writeConfig(
      join(home, '.config', 'svcview'),
      workersWithStatusCommand(`[cat, '{{.Env.WORKER_STATUS_FILE}}']`),
      0o666,
    )

    const result = await runCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })

    expect(result.stdout).toContain(path)
    expect(result.stdout).toContain('another user can write')
    expect(result.stdout).not.toContain('Template values')
    expect(result.status).toBe(1)
  })

  it('svcview --help lists status', async () => {
    const result = await runCli(scratchDir(), ['--help'])

    expect(result.stdout).toMatch(/^ +status +\S/m)
    expect(result.status).toBe(0)
  })

  it('in a terminal, prints the template values and the result, and ends with the exit code 0, with no screen', async () => {
    const { home, statusFile } = homeWithStatusFile('{"version":1,"summary":"busy"}\n')
    writeConfig(join(home, '.config', 'svcview'), workersWithStatusCommand(`[cat, '{{.Env.WORKER_STATUS_FILE}}']`))

    const session = await openCli(scratchDir(), ['status', 'com.example.worker'], { HOME: home })

    await session.waitForText('EXIT:0')
    const printed = (await session.text()).replaceAll('\n', '')
    expect(printed).toContain('Template values')
    expect(printed).toContain(`{{.Env.WORKER_STATUS_FILE}}  ${statusFile}`)
    expect(printed).toContain('valid')
    expect(printed).not.toContain('╭ Services')
  })
})
