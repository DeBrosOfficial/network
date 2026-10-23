// client.network against the public gateway (bug-724). connect and disconnect
// are an operator's: the namespace owner's session holds the operator domain
// through the owner grant and is refused by the operator list, before the
// body is read (docs/whitepaper/technical-reference/appendices/i-api-surface.md#network-and-proxy). The body
// the SDK sends ({multiaddr}, {peer_id}) is pinned by contracts/network/.
import { AuthCode, ScopeError } from "../../../../sdk/src/index";
import { mainURL, need, nsClient, onFleet } from "./fleet";

const notOperator = { httpStatus: 403, code: AuthCode.OperatorRequired };

describe.skipIf(!onFleet)("sdk network topology mutation", () => {
  const owner = () => nsClient({ baseURL: mainURL(), jwt: need("GATEWAY_JWT") });

  it("connect from a namespace owner who is not an operator is NOT_AN_OPERATOR", async () => {
    const err = await owner()
      .network.connect("/ip4/10.0.0.250/tcp/4001/p2p/12D3KooWGzBLmKGtJ4TJ7XFUH8uPEM1L6Nwrn4UtqFLy3RqvQoHb")
      .catch((e) => e);
    expect(err).toBeInstanceOf(ScopeError);
    expect(err).toMatchObject(notOperator);
  });

  it("disconnect from a namespace owner who is not an operator is NOT_AN_OPERATOR", async () => {
    const err = await owner()
      .network.disconnect("12D3KooWGzBLmKGtJ4TJ7XFUH8uPEM1L6Nwrn4UtqFLy3RqvQoHb")
      .catch((e) => e);
    expect(err).toBeInstanceOf(ScopeError);
    expect(err).toMatchObject(notOperator);
  });
});
