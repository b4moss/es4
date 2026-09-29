import { describe, expect, it } from "vitest";
import {
  decodeFirestoreDocID,
  encodeFirestoreDocID,
} from "../src/state/firestore.js";
import { ErrFirestoreDocIDTooLong, isEs4Error } from "../src/errors.js";

describe("Firestore key mapping K-N*", () => {
  it("K-N encode a/b/c → a%2Fb%2Fc", () => {
    expect(encodeFirestoreDocID("a/b/c")).toBe("a%2Fb%2Fc");
    expect(decodeFirestoreDocID("a%2Fb%2Fc")).toBe("a/b/c");
  });

  it("K-E doc id too long", () => {
    const long = "x".repeat(1600);
    expect(() => encodeFirestoreDocID(long)).toThrow();
    try {
      encodeFirestoreDocID(long);
    } catch (e) {
      expect(isEs4Error(e, ErrFirestoreDocIDTooLong)).toBe(true);
    }
  });
});
