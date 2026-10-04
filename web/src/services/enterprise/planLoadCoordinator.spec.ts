import { describe, expect, it, vi } from 'vitest'
import { createPlanLoadCoordinator } from './planLoadCoordinator'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (cause: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
const handlers = () => ({ start: vi.fn(), success: vi.fn(), failure: vi.fn(), finish: vi.fn() })

describe('plan read coordination', () => {
  it('coalesces rapid repeated reads in one scope', async () => {
    const load = createPlanLoadCoordinator<number>()
    const response = deferred<number>()
    const read = vi.fn(() => response.promise)
    const sink = handlers()
    const first = load.run('tenant-A:session-1', read, () => 'tenant-A:session-1', sink)
    const second = load.run('tenant-A:session-1', read, () => 'tenant-A:session-1', sink)
    expect(first).toBe(second)
    response.resolve(18)
    await first
    expect(read).toHaveBeenCalledTimes(1)
    expect(sink.success).toHaveBeenCalledWith(18)
    expect(sink.finish).toHaveBeenCalledTimes(1)
  })
  it('discards a late success after a tenant switch', async () => {
    const load = createPlanLoadCoordinator<string>()
    let scope = 'A'
    const old = deferred<string>()
    const oldSink = handlers()
    const task = load.run(scope, () => old.promise, () => scope, oldSink)
    load.invalidate()
    scope = 'B'
    const newSink = handlers()
    await load.run(scope, async () => 'B plan', () => scope, newSink)
    old.resolve('A plan')
    await task
    expect(oldSink.success).not.toHaveBeenCalled()
    expect(oldSink.finish).not.toHaveBeenCalled()
    expect(newSink.success).toHaveBeenCalledWith('B plan')
  })
  it('does not let an old failure overwrite a newer successful read', async () => {
    const load = createPlanLoadCoordinator<string>()
    const old = deferred<string>()
    const oldSink = handlers()
    const first = load.run('A:1', () => old.promise, () => 'A:1', oldSink)
    load.invalidate()
    const newSink = handlers()
    await load.run('A:2', async () => 'new', () => 'A:2', newSink)
    old.reject(new Error('old failure'))
    await expect(first).resolves.toBeUndefined()
    expect(oldSink.failure).not.toHaveBeenCalled()
    expect(newSink.success).toHaveBeenCalledWith('new')
  })
  it('handles a failed read without an unhandled rejection and permits a safe new read', async () => {
    const load = createPlanLoadCoordinator<number>()
    const sink = handlers()
    await expect(load.run('A', async () => { throw new Error('offline') }, () => 'A', sink)).resolves.toBeUndefined()
    expect(sink.failure).toHaveBeenCalledTimes(1)
    await load.run('A', async () => 20, () => 'A', sink)
    expect(sink.success).toHaveBeenCalledWith(20)
    expect(sink.finish).toHaveBeenCalledTimes(2)
  })
  it('drops a result even when the scope changes without an explicit invalidation', async () => {
    const load = createPlanLoadCoordinator<number>()
    const response = deferred<number>()
    let key = 'A:1'
    const sink = handlers()
    const task = load.run(key, () => response.promise, () => key, sink)
    key = 'A:2'
    response.resolve(1)
    await task
    expect(sink.success).not.toHaveBeenCalled()
  })
})
