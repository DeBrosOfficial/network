// The pubsub WebSocket and session revocation: ending a session closes the
// sockets its tokens hold open within ten seconds (docs/whitepaper/technical-reference/vol1/13-identity.md "Which
// machines are signed in"), and the SDK reports it.
import { createClient, MemoryStorage, PubSubClient } from "../../../../sdk/src/index";
import { http, need, nsClient, nsURL, onFleet, unique, until } from "./fleet";

const revocationWindowMs = 10_000;

describe.skipIf(!onFleet)("sdk websocket", () => {
  it("a subscription receives what is published, and closes when its session is ended", async () => {
    const storage = new MemoryStorage();
    await storage.set("refreshToken", need("E2E_WS_REFRESH"));
    await storage.set("namespace", need("E2E_NAMESPACE"));
    const client = nsClient({ jwt: need("E2E_WS_JWT"), storage, wsConfig: { reconnect: { enabled: false } } });
    const topic = unique("e2e-ws");
    const received: string[] = [];
    let closed = false;
    const sub = await client.pubsub.subscribe(topic, {
      onMessage: (m: any) => received.push(typeof m.data === "string" ? m.data : JSON.stringify(m.data)),
    });
    sub.onClose(() => {
      closed = true;
    });
    try {
      await client.pubsub.publish(topic, "hello e2e");
      await until("the published message", () => received.length > 0, 15_000);
      expect(received.join("")).toContain("hello e2e");

      const ender = createClient({ baseURL: need("GATEWAY_BASE_URL"), jwt: need("E2E_WS_JWT") });
      await ender.auth.logout();
      await until("the gateway closing the revoked session's socket", () => closed, revocationWindowMs);
    } finally {
      sub.close();
    }
  });

  // A PubSubClient built on an HttpClient alone, with no wsURL, used to dial
  // ws://127.0.0.1:6001. It now subscribes on the gateway its HTTP client talks to.
  it("a PubSubClient with no wsURL subscribes on its HTTP client's gateway", async () => {
    const pubsub = new PubSubClient(http(nsURL(), need("GATEWAY_JWT")), { reconnect: { enabled: false } });
    const topic = unique("e2e-ws-direct");
    const received: string[] = [];
    const sub = await pubsub.subscribe(topic, {
      onMessage: (m: any) => received.push(typeof m.data === "string" ? m.data : JSON.stringify(m.data)),
    });
    try {
      await pubsub.publish(topic, "direct");
      await until("the published message on the derived WebSocket URL", () => received.length > 0, 15_000);
      expect(received.join("")).toContain("direct");
    } finally {
      sub.close();
    }
  });
});
