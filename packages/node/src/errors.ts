/** Sentinel errors matching Go `state` / `es4` names and messages. */

export class Es4Error extends Error {
  readonly code: string;
  constructor(code: string, message: string) {
    super(message);
    this.name = "Es4Error";
    this.code = code;
  }
}

export const ErrNotFound = Object.freeze(
  new Es4Error("ErrNotFound", "state: key not found"),
);
export const ErrInvalidKey = Object.freeze(
  new Es4Error("ErrInvalidKey", "state: invalid key"),
);
export const ErrInvalidValue = Object.freeze(
  new Es4Error("ErrInvalidValue", "state: invalid JSON value"),
);
export const ErrClosed = Object.freeze(
  new Es4Error("ErrClosed", "state: closed"),
);
export const ErrTxDone = Object.freeze(
  new Es4Error("ErrTxDone", "state: transaction finished"),
);
export const ErrNestedTx = Object.freeze(
  new Es4Error("ErrNestedTx", "state: nested transaction not supported"),
);
export const ErrFirestoreDocIDTooLong = Object.freeze(
  new Es4Error(
    "ErrFirestoreDocIDTooLong",
    "state: firestore document id too long",
  ),
);
export const ErrRecoveryNotFound = Object.freeze(
  new Es4Error("ErrRecoveryNotFound", "recovery: not found"),
);

export function isEs4Error(err: unknown, sentinel: Es4Error): boolean {
  if (err === sentinel) return true;
  if (!(err instanceof Error)) return false;
  if (err instanceof Es4Error && err.code === sentinel.code) return true;
  const cause = (err as Error & { cause?: unknown }).cause;
  if (cause !== undefined) return isEs4Error(cause, sentinel);
  return err.message.includes(sentinel.message);
}

export function isNotFound(err: unknown): boolean {
  return isEs4Error(err, ErrNotFound);
}

export function wrapError(sentinel: Es4Error, detail: string): Error {
  const e = new Es4Error(sentinel.code, `${sentinel.message}: ${detail}`);
  e.cause = sentinel;
  return e;
}

export function throwIfAborted(signal?: AbortSignal): void {
  if (signal?.aborted) {
    const reason = signal.reason;
    if (reason instanceof Error) throw reason;
    throw new Error("aborted", { cause: reason });
  }
}
