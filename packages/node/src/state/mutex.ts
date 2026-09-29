/** Simple async mutex with condition-style wait for active Tx. */

export class AsyncMutex {
  private locked = false;
  private readonly waiters: Array<() => void> = [];

  async acquire(): Promise<() => void> {
    while (this.locked) {
      await new Promise<void>((resolve) => this.waiters.push(resolve));
    }
    this.locked = true;
    return () => {
      this.locked = false;
      const next = this.waiters.shift();
      if (next) next();
    };
  }

  async withLock<T>(fn: () => Promise<T> | T): Promise<T> {
    const release = await this.acquire();
    try {
      return await fn();
    } finally {
      release();
    }
  }
}

/** Wait while predicate is true (must be called under mutex with release/reacquire). */
export async function waitWhile(
  mutex: AsyncMutex,
  release: () => void,
  predicate: () => boolean,
): Promise<() => void> {
  while (predicate()) {
    release();
    await new Promise((r) => setTimeout(r, 1));
    release = await mutex.acquire();
  }
  return release;
}
