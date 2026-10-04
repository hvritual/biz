/** Collapses identical reads and makes responses from a replaced scope inert. */
export function createPlanLoadCoordinator<T>() {
  let generation = 0
  let pending: { key: string; task: Promise<void> } | null = null

  function invalidate() {
    generation += 1
    pending = null
  }

  function run(
    key: string,
    read: () => Promise<T>,
    currentKey: () => string,
    handlers: { start: () => void; success: (value: T) => void; failure: (cause: unknown) => void; finish: () => void },
  ): Promise<void> {
    if (pending?.key === key) return pending.task
    const ticket = ++generation
    handlers.start()
    const isCurrent = () => ticket === generation && key === currentKey()
    // Start after pending is installed; even a synchronously throwing adapter
    // goes through failure/finally and does not poison the single-flight slot.
    const task = Promise.resolve().then(read).then(
      (value) => { if (isCurrent()) handlers.success(value) },
    ).catch((cause: unknown) => {
      if (isCurrent()) handlers.failure(cause)
    }).finally(() => {
      if (ticket === generation) pending = null
      if (isCurrent()) handlers.finish()
    })
    pending = { key, task }
    return task
  }

  return { run, invalidate }
}
