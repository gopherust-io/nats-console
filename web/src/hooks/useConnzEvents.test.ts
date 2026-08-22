import { describe, expect, it } from "vitest";
import { connzMembershipKey } from "./useConnzEvents";

describe("connzMembershipKey", () => {
  it("changes when a connection is added or removed", () => {
    const one = connzMembershipKey({
      num_connections: 1,
      connections: [{ cid: 10 }],
    });
    const two = connzMembershipKey({
      num_connections: 2,
      connections: [{ cid: 10 }, { cid: 11 }],
    });
    const back = connzMembershipKey({
      num_connections: 1,
      connections: [{ cid: 11 }],
    });
    expect(one).not.toEqual(two);
    expect(two).not.toEqual(back);
    expect(one).not.toEqual(back);
  });

  it("is order-independent for the same cid set", () => {
    const a = connzMembershipKey({
      connections: [{ cid: 2 }, { cid: 1 }],
    });
    const b = connzMembershipKey({
      connections: [{ cid: 1 }, { cid: 2 }],
    });
    expect(a).toEqual(b);
  });
});
