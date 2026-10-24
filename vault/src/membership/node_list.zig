/// Node list management — discovers guardians through the index RQLite.
///
/// Every node in the Orama network runs a vault guardian, so the guardian set
/// is the cluster's active nodes (`dns_nodes`).
const std = @import("std");
const log = @import("../log.zig");

pub const Node = struct {
    /// WireGuard IP address (e.g., "10.0.0.1")
    address: []const u8,
    /// Vault guardian client port
    port: u16,
    /// Whether this node is reachable
    state: NodeState,
    /// Last successful heartbeat (nanos since epoch)
    last_seen_ns: i128,
};

pub const NodeState = enum {
    alive,
    suspect,
    dead,
    unknown,
};

pub const NodeList = struct {
    nodes: []Node,
    allocator: std.mem.Allocator,
    /// Self node index (-1 if not identified)
    self_index: ?usize,

    pub fn deinit(self: *NodeList) void {
        for (self.nodes) |node| {
            self.allocator.free(node.address);
        }
        self.allocator.free(self.nodes);
    }

    /// Number of alive nodes.
    pub fn aliveCount(self: *const NodeList) usize {
        var count: usize = 0;
        for (self.nodes) |node| {
            if (node.state == .alive) count += 1;
        }
        return count;
    }

    /// Update a node's state.
    pub fn updateState(self: *NodeList, address: []const u8, port: u16, state: NodeState) void {
        for (self.nodes) |*node| {
            if (std.mem.eql(u8, node.address, address) and node.port == port) {
                node.state = state;
                if (state == .alive) {
                    node.last_seen_ns = std.time.nanoTimestamp();
                }
                return;
            }
        }
    }

    /// Replace this list with `fresh` (allocated with the same allocator),
    /// keeping the state and last-seen time of every node in both lists so a
    /// refresh does not forget what heartbeats learned. The node at
    /// `self_address` becomes self and alive; when this guardian is not among
    /// the active nodes there is no self.
    pub fn adopt(self: *NodeList, fresh: NodeList, self_address: []const u8) void {
        var next = fresh;
        next.self_index = null;
        for (next.nodes, 0..) |*node, i| {
            for (self.nodes) |old| {
                if (std.mem.eql(u8, old.address, node.address) and old.port == node.port) {
                    node.state = old.state;
                    node.last_seen_ns = old.last_seen_ns;
                    break;
                }
            }
            if (std.mem.eql(u8, node.address, self_address)) {
                next.self_index = i;
                node.state = .alive;
                node.last_seen_ns = std.time.nanoTimestamp();
            }
        }
        self.deinit();
        self.* = next;
    }

    /// Get all alive nodes except self.
    pub fn peers(self: *const NodeList, allocator: std.mem.Allocator) ![]Node {
        var list = std.ArrayListUnmanaged(Node){};
        errdefer list.deinit(allocator);

        for (self.nodes, 0..) |node, i| {
            if (self.self_index != null and i == self.self_index.?) continue;
            if (node.state == .alive) {
                try list.append(allocator, node);
            }
        }
        return list.toOwnedSlice(allocator);
    }
};

/// The guardian set is every active node of the cluster, read from the index
/// RQLite's `dns_nodes` table: the same rows, and the same address choice
/// (WireGuard address, else the public one), the gateway reads when it places
/// shares (core/pkg/gateway/handlers/vault/handlers.go:discoverGuardians).
/// A row goes inactive after 120 s without a node heartbeat.
pub const active_nodes_query = "SELECT COALESCE(internal_ip, ip_address) FROM dns_nodes WHERE status = 'active'";

/// Upper bound on the RQLite answer. One row is one address, about 30 bytes of
/// JSON, so 1 MiB holds some 30,000 nodes: two orders of magnitude above the
/// largest cluster, and still a bound on what a misbehaving peer can make the
/// guardian buffer.
pub const max_response_bytes: usize = 1 << 20;

/// Credentials for an RQLite that runs with `-auth` (the cluster's `orama`
/// user, whose password is in secrets/rqlite-password).
pub const RqliteAuth = struct {
    username: []const u8,
    password: []const u8,
};

pub const FetchError = error{
    /// RQLite answered with a status other than 200.
    RqliteStatus,
    /// RQLite answered 200 with an error for the statement.
    RqliteQueryFailed,
    /// The answer was not the JSON RQLite writes.
    RqliteMalformedResponse,
};

/// Fetch the active nodes from RQLite at `rqlite_url` (http://host:port),
/// each paired with `vault_port`.
///
/// The read is `level=none`: answered by the RQLite the guardian asked, with
/// no leader round trip and no redirect. Membership changes on the scale of
/// the 120 s heartbeat timeout, so a follower's slightly stale copy is enough.
pub fn fetchFromRqlite(
    allocator: std.mem.Allocator,
    rqlite_url: []const u8,
    auth: ?RqliteAuth,
    vault_port: u16,
) !NodeList {
    const url = try std.fmt.allocPrint(allocator, "{s}/db/query?level=none", .{std.mem.trimRight(u8, rqlite_url, "/")});
    defer allocator.free(url);

    var auth_header: ?[]u8 = null;
    defer if (auth_header) |h| allocator.free(h);
    if (auth) |a| auth_header = try basicAuthHeader(allocator, a);

    const body = try std.json.Stringify.valueAlloc(allocator, [_][]const u8{active_nodes_query}, .{});
    defer allocator.free(body);

    const response_buf = try allocator.alloc(u8, max_response_bytes);
    defer allocator.free(response_buf);
    var response_writer = std.Io.Writer.fixed(response_buf);

    var client: std.http.Client = .{ .allocator = allocator };
    defer client.deinit();

    const result = try client.fetch(.{
        .location = .{ .url = url },
        .method = .POST,
        .payload = body,
        .keep_alive = false,
        .headers = .{
            .content_type = .{ .override = "application/json" },
            .authorization = if (auth_header) |h| .{ .override = h } else .omit,
        },
        .response_writer = &response_writer,
    });
    if (result.status != .ok) {
        log.warn("rqlite answered {d} to the guardian list query", .{@intFromEnum(result.status)});
        return FetchError.RqliteStatus;
    }
    return parseActiveNodes(allocator, response_writer.buffered(), vault_port);
}

/// The `Authorization` value for HTTP basic auth.
fn basicAuthHeader(allocator: std.mem.Allocator, auth: RqliteAuth) ![]u8 {
    const plain = try std.fmt.allocPrint(allocator, "{s}:{s}", .{ auth.username, auth.password });
    defer {
        std.crypto.secureZero(u8, plain);
        allocator.free(plain);
    }
    const enc = std.base64.standard.Encoder;
    const header = try allocator.alloc(u8, "Basic ".len + enc.calcSize(plain.len));
    @memcpy(header[0.."Basic ".len], "Basic ");
    _ = enc.encode(header["Basic ".len..], plain);
    return header;
}

/// Parse RQLite's answer to `active_nodes_query` into a node list.
///
/// A NULL or empty address is skipped, as the gateway skips it. An address that
/// is not an IP is skipped and logged: it names nothing a guardian could
/// reach. A repeated address is listed once.
pub fn parseActiveNodes(allocator: std.mem.Allocator, body: []const u8, vault_port: u16) !NodeList {
    const Response = struct {
        results: []const struct {
            values: ?[]const []const ?[]const u8 = null,
            @"error": ?[]const u8 = null,
        },
    };
    const parsed = std.json.parseFromSlice(Response, allocator, body, .{ .ignore_unknown_fields = true }) catch {
        return FetchError.RqliteMalformedResponse;
    };
    defer parsed.deinit();

    if (parsed.value.results.len != 1) return FetchError.RqliteMalformedResponse;
    const result = parsed.value.results[0];
    if (result.@"error") |msg| {
        log.warn("rqlite refused the guardian list query: {s}", .{msg});
        return FetchError.RqliteQueryFailed;
    }

    var list = std.ArrayListUnmanaged(Node){};
    errdefer {
        for (list.items) |node| allocator.free(node.address);
        list.deinit(allocator);
    }
    rows: for (result.values orelse &.{}) |row| {
        if (row.len == 0) continue;
        const address = row[0] orelse continue;
        if (address.len == 0) continue;
        _ = std.net.Address.parseIp(address, vault_port) catch {
            log.warn("dns_nodes holds {s}, which is not an IP address; skipped", .{address});
            continue;
        };
        for (list.items) |existing| {
            if (std.mem.eql(u8, existing.address, address)) continue :rows;
        }
        const owned = try allocator.dupe(u8, address);
        errdefer allocator.free(owned);
        try list.append(allocator, Node{
            .address = owned,
            .port = vault_port,
            .state = .unknown,
            .last_seen_ns = 0,
        });
    }

    return NodeList{
        .nodes = try list.toOwnedSlice(allocator),
        .allocator = allocator,
        .self_index = null,
    };
}

/// A list with no node: the guardian's view before the first fetch succeeds.
pub fn empty(allocator: std.mem.Allocator) !NodeList {
    return NodeList{
        .nodes = try allocator.alloc(Node, 0),
        .allocator = allocator,
        .self_index = null,
    };
}

/// Create a node list from a static list of addresses (tests).
pub fn fromStatic(allocator: std.mem.Allocator, addresses: []const []const u8, port: u16) !NodeList {
    const nodes = try allocator.alloc(Node, addresses.len);
    errdefer allocator.free(nodes);

    for (addresses, 0..) |addr, i| {
        nodes[i] = Node{
            .address = try allocator.dupe(u8, addr),
            .port = port,
            .state = .unknown,
            .last_seen_ns = 0,
        };
    }

    return NodeList{
        .nodes = nodes,
        .allocator = allocator,
        .self_index = null,
    };
}

// ── Tests ────────────────────────────────────────────────────────────────────

test "node_list: static creation" {
    const allocator = std.testing.allocator;
    const addrs = [_][]const u8{ "10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5" };
    var nl = try fromStatic(allocator, &addrs, 7500);
    defer nl.deinit();

    try std.testing.expectEqual(@as(usize, 5), nl.nodes.len);
    try std.testing.expectEqual(@as(usize, 0), nl.aliveCount());
}

test "node_list: alive count" {
    const allocator = std.testing.allocator;
    const addrs = [_][]const u8{
        "10.0.0.1",  "10.0.0.2",  "10.0.0.3",  "10.0.0.4",  "10.0.0.5",
        "10.0.0.6",  "10.0.0.7",  "10.0.0.8",  "10.0.0.9",  "10.0.0.10",
        "10.0.0.11", "10.0.0.12", "10.0.0.13", "10.0.0.14",
    };
    var nl = try fromStatic(allocator, &addrs, 7500);
    defer nl.deinit();

    // Mark all alive
    for (nl.nodes) |*node| {
        node.state = .alive;
    }

    try std.testing.expectEqual(@as(usize, 14), nl.aliveCount());
}

test "node_list: updateState" {
    const allocator = std.testing.allocator;
    const addrs = [_][]const u8{ "10.0.0.1", "10.0.0.2" };
    var nl = try fromStatic(allocator, &addrs, 7500);
    defer nl.deinit();

    nl.updateState("10.0.0.1", 7500, .alive);
    try std.testing.expectEqual(NodeState.alive, nl.nodes[0].state);
    try std.testing.expectEqual(NodeState.unknown, nl.nodes[1].state);
    try std.testing.expectEqual(@as(usize, 1), nl.aliveCount());

    nl.updateState("10.0.0.1", 7500, .dead);
    try std.testing.expectEqual(NodeState.dead, nl.nodes[0].state);
    try std.testing.expectEqual(@as(usize, 0), nl.aliveCount());
}

test "node_list: peers excludes self" {
    const allocator = std.testing.allocator;
    const addrs = [_][]const u8{ "10.0.0.1", "10.0.0.2", "10.0.0.3" };
    var nl = try fromStatic(allocator, &addrs, 7500);
    defer nl.deinit();

    nl.self_index = 1; // we are 10.0.0.2
    for (nl.nodes) |*node| node.state = .alive;

    const peer_list = try nl.peers(allocator);
    defer allocator.free(peer_list);

    try std.testing.expectEqual(@as(usize, 2), peer_list.len);
}

test "node_list: an empty list" {
    const allocator = std.testing.allocator;
    var nl = try empty(allocator);
    defer nl.deinit();
    try std.testing.expectEqual(@as(usize, 0), nl.nodes.len);
}

test "node_list: parseActiveNodes reads every active address once" {
    const allocator = std.testing.allocator;
    const body =
        \\{"results":[{"columns":["COALESCE(internal_ip, ip_address)"],"types":["text"],
        \\"values":[["10.0.0.1"],["10.0.0.2"],[null],[""],["not-an-ip"],["10.0.0.1"],["2001:db8::1"]]}],"time":0.001}
    ;
    var nl = try parseActiveNodes(allocator, body, 10106);
    defer nl.deinit();

    try std.testing.expectEqual(@as(usize, 3), nl.nodes.len);
    try std.testing.expectEqualStrings("10.0.0.1", nl.nodes[0].address);
    try std.testing.expectEqualStrings("10.0.0.2", nl.nodes[1].address);
    try std.testing.expectEqualStrings("2001:db8::1", nl.nodes[2].address);
    for (nl.nodes) |node| {
        try std.testing.expectEqual(@as(u16, 10106), node.port);
        try std.testing.expectEqual(NodeState.unknown, node.state);
    }
}

test "node_list: parseActiveNodes with no active node" {
    const allocator = std.testing.allocator;
    var nl = try parseActiveNodes(allocator, "{\"results\":[{\"columns\":[\"a\"],\"types\":[\"text\"]}]}", 10106);
    defer nl.deinit();
    try std.testing.expectEqual(@as(usize, 0), nl.nodes.len);
}

test "node_list: parseActiveNodes refuses an error or a foreign body" {
    const allocator = std.testing.allocator;
    try std.testing.expectError(FetchError.RqliteQueryFailed, parseActiveNodes(allocator, "{\"results\":[{\"error\":\"no such table: dns_nodes\"}]}", 10106));
    try std.testing.expectError(FetchError.RqliteMalformedResponse, parseActiveNodes(allocator, "<html>401</html>", 10106));
    try std.testing.expectError(FetchError.RqliteMalformedResponse, parseActiveNodes(allocator, "{\"results\":[]}", 10106));
}

test "node_list: adopt keeps what heartbeats learned and finds self" {
    const allocator = std.testing.allocator;
    const before = [_][]const u8{ "10.0.0.1", "10.0.0.2" };
    var nl = try fromStatic(allocator, &before, 10106);
    defer nl.deinit();
    nl.nodes[1].state = .suspect;
    nl.nodes[1].last_seen_ns = 42;

    const after = [_][]const u8{ "10.0.0.2", "10.0.0.3" };
    nl.adopt(try fromStatic(allocator, &after, 10106), "10.0.0.3");

    try std.testing.expectEqual(@as(usize, 2), nl.nodes.len);
    try std.testing.expectEqual(NodeState.suspect, nl.nodes[0].state);
    try std.testing.expectEqual(@as(i128, 42), nl.nodes[0].last_seen_ns);
    try std.testing.expectEqual(@as(?usize, 1), nl.self_index);
    try std.testing.expectEqual(NodeState.alive, nl.nodes[1].state);

    nl.adopt(try fromStatic(allocator, &before, 10106), "10.0.0.3");
    try std.testing.expectEqual(@as(?usize, null), nl.self_index);
}

/// Serves one HTTP request on `server` with `reply`, recording the request.
fn serveOnce(server: *std.net.Server, reply: []const u8, request: *[4096]u8, request_len: *usize) void {
    const conn = server.accept() catch return;
    defer conn.stream.close();
    var total: usize = 0;
    while (total < request.len) {
        const n = conn.stream.read(request[total..]) catch return;
        if (n == 0) break;
        total += n;
        const head_end = std.mem.indexOf(u8, request[0..total], "\r\n\r\n") orelse continue;
        const head = request[0..head_end];
        const cl_at = std.ascii.indexOfIgnoreCase(head, "content-length: ") orelse break;
        const cl_end = std.mem.indexOfScalarPos(u8, head, cl_at, '\r') orelse head.len;
        const cl = std.fmt.parseInt(usize, head[cl_at + "content-length: ".len .. cl_end], 10) catch break;
        if (total >= head_end + 4 + cl) break;
    }
    request_len.* = total;
    conn.stream.writeAll(reply) catch {};
}

test "node_list: fetchFromRqlite asks rqlite for the active nodes, authenticated" {
    const allocator = std.testing.allocator;
    const address = try std.net.Address.parseIp("127.0.0.1", 0);
    var server = try address.listen(.{ .reuse_address = true });
    defer server.deinit();

    const json = "{\"results\":[{\"columns\":[\"a\"],\"types\":[\"text\"],\"values\":[[\"10.0.0.7\"],[\"10.0.0.8\"]]}]}";
    const reply = std.fmt.comptimePrint("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {d}\r\nConnection: close\r\n\r\n{s}", .{ json.len, json });
    var request: [4096]u8 = undefined;
    var request_len: usize = 0;
    const thread = try std.Thread.spawn(.{}, serveOnce, .{ &server, reply, &request, &request_len });

    const url = try std.fmt.allocPrint(allocator, "http://127.0.0.1:{d}", .{server.listen_address.getPort()});
    defer allocator.free(url);
    var nl = try fetchFromRqlite(allocator, url, .{ .username = "orama", .password = "s3cret" }, 10106);
    defer nl.deinit();
    thread.join();

    try std.testing.expectEqual(@as(usize, 2), nl.nodes.len);
    try std.testing.expectEqualStrings("10.0.0.7", nl.nodes[0].address);

    const sent = request[0..request_len];
    try std.testing.expect(std.mem.startsWith(u8, sent, "POST /db/query?level=none HTTP/1.1\r\n"));
    // base64("orama:s3cret")
    try std.testing.expect(std.mem.indexOf(u8, sent, "Basic b3JhbWE6czNjcmV0") != null);
    try std.testing.expect(std.mem.indexOf(u8, sent, "FROM dns_nodes WHERE status = 'active'") != null);
}

test "node_list: fetchFromRqlite refuses a non-200 answer" {
    const allocator = std.testing.allocator;
    const address = try std.net.Address.parseIp("127.0.0.1", 0);
    var server = try address.listen(.{ .reuse_address = true });
    defer server.deinit();

    const reply = "HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\nConnection: close\r\n\r\n";
    var request: [4096]u8 = undefined;
    var request_len: usize = 0;
    const thread = try std.Thread.spawn(.{}, serveOnce, .{ &server, reply, &request, &request_len });

    const url = try std.fmt.allocPrint(allocator, "http://127.0.0.1:{d}", .{server.listen_address.getPort()});
    defer allocator.free(url);
    const result = fetchFromRqlite(allocator, url, null, 10106);
    thread.join();
    try std.testing.expectError(FetchError.RqliteStatus, result);
    try std.testing.expect(std.mem.indexOf(u8, request[0..request_len], "Authorization") == null);
}
