/// GET /v1/vault/guardians — List active guardians.
///
/// Reports the alive guardians, the read threshold and the total from the
/// Guardian's node list.
const std = @import("std");
const response = @import("response.zig");
const router = @import("router.zig");
const guardian_mod = @import("../guardian.zig");

pub fn handle(writer: anytype, ctx: *const router.RouteContext) !void {
    if (ctx.guardian) |guardian| {
        // The body grows with the cluster (about 40 bytes a guardian), so it
        // is built on the heap rather than in a fixed buffer.
        var body = std.ArrayListUnmanaged(u8){};
        defer body.deinit(ctx.allocator);
        buildBody(ctx.allocator, &body, guardian) catch {
            return response.internalError(writer);
        };
        try response.jsonOk(writer, body.items);
    } else {
        // Fallback: no guardian available, report self only
        var buf: [512]u8 = undefined;
        const body = std.fmt.bufPrint(&buf,
            \\{{"guardians":[{{"address":"{s}","port":{d}}}],"threshold":3,"total":1}}
        , .{ ctx.listen_address, ctx.client_port }) catch {
            return response.internalError(writer);
        };
        try response.jsonOk(writer, body);
    }
}

/// The JSON body: alive guardians, the read threshold and the total, read
/// under the guardian's node lock.
fn buildBody(allocator: std.mem.Allocator, body: *std.ArrayListUnmanaged(u8), guardian: *guardian_mod.Guardian) !void {
    guardian.nodes_mu.lock();
    defer guardian.nodes_mu.unlock();

    const w = body.writer(allocator);
    try w.writeAll("{\"guardians\":[");
    var first = true;
    for (guardian.nodes.nodes) |node| {
        if (node.state != .alive) continue;
        if (!first) try w.writeAll(",");
        first = false;
        try w.print("{{\"address\":\"{s}\",\"port\":{d}}}", .{ node.address, node.port });
    }
    try w.print("],\"threshold\":{d},\"total\":{d}}}", .{ guardian.readThreshold(), guardian.nodes.nodes.len });
}
