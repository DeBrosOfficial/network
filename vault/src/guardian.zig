/// Top-level Guardian struct — orchestrates all subsystems.
///
/// The Guardian is the main runtime object that ties together:
/// - HTTP server (client-facing, port 7500)
/// - Peer protocol (guardian-to-guardian, port 7501)
/// - Node membership (the cluster's active nodes, read from the index RQLite)
/// - Heartbeat/health management
/// - Storage operations
const std = @import("std");
const log = @import("log.zig");
const config = @import("config.zig");
const node_list = @import("membership/node_list.zig");
const quorum = @import("membership/quorum.zig");
const heartbeat = @import("peer/heartbeat.zig");

/// How often the guardian list is re-read from RQLite. A node's dns_nodes row
/// goes inactive after 120 s without a heartbeat, so 30 s follows membership
/// closely; one indexed query per guardian every 30 s is nothing for RQLite at
/// any cluster size.
pub const discovery_interval_ns: i128 = 30 * std.time.ns_per_s;

/// Upper bound on the RQLite password file.
const max_password_bytes = 4096;

pub const DiscoveryConfigError = error{
    /// rqlite_username is set without rqlite_password_file.
    RqlitePasswordFileMissing,
};

pub const Guardian = struct {
    cfg: config.Config,
    /// The guardian list. The HTTP thread reads it and the housekeeping
    /// thread replaces it, so every access holds nodes_mu.
    nodes: node_list.NodeList,
    nodes_mu: std.Thread.Mutex = .{},
    allocator: std.mem.Allocator,
    /// Random server secret for HMAC-based auth (generated at startup).
    /// Ephemeral by design: regenerated each boot and used only to sign
    /// session/challenge tokens, which clients re-acquire on demand.
    server_secret: [32]u8,
    /// Persistent at-rest integrity key, loaded/created in data_dir. Keys the
    /// HMAC over stored share files, so it MUST survive restarts — otherwise
    /// every stored share fails its integrity check after a reboot.
    integrity_key: [32]u8,
    /// Share count cache (refreshed periodically)
    share_count: u32,

    pub fn init(allocator: std.mem.Allocator, cfg: config.Config) !Guardian {
        // Generate ephemeral server secret (auth tokens only).
        var secret: [32]u8 = undefined;
        std.crypto.random.bytes(&secret);

        // Load (or create) the persistent at-rest integrity key.
        const integrity_key = loadOrCreateIntegrityKey(cfg.data_dir, allocator);

        const share_count = heartbeat.countShares(cfg.data_dir);

        var g = Guardian{
            .cfg = cfg,
            .nodes = try node_list.empty(allocator),
            .allocator = allocator,
            .server_secret = secret,
            .integrity_key = integrity_key,
            .share_count = share_count,
        };
        // RQLite may not be up yet when the guardian starts; the list stays
        // empty until the housekeeping thread's next refresh reads it.
        g.refreshNodes() catch |err| {
            log.warn("guardian list not read from RQLite ({}); next attempt in {d} s", .{ err, @divTrunc(discovery_interval_ns, std.time.ns_per_s) });
        };
        return g;
    }

    /// Re-read the guardian list from RQLite and adopt it, keeping each known
    /// node's heartbeat state. On an error the current list is kept.
    pub fn refreshNodes(self: *Guardian) !void {
        const password = try self.readRqlitePassword();
        defer if (password) |p| {
            std.crypto.secureZero(u8, p);
            self.allocator.free(p);
        };
        const auth: ?node_list.RqliteAuth = if (self.cfg.rqlite_username) |user|
            .{ .username = user, .password = std.mem.trim(u8, password.?, " \t\r\n") }
        else
            null;

        const fresh = try node_list.fetchFromRqlite(self.allocator, self.cfg.rqlite_url, auth, self.cfg.client_port);
        self.nodes_mu.lock();
        defer self.nodes_mu.unlock();
        self.nodes.adopt(fresh, self.cfg.listen_address);
    }

    /// The RQLite password, or null when no user is configured.
    fn readRqlitePassword(self: *const Guardian) !?[]u8 {
        if (self.cfg.rqlite_username == null) return null;
        const path = self.cfg.rqlite_password_file orelse return DiscoveryConfigError.RqlitePasswordFileMissing;
        return try std.fs.cwd().readFileAlloc(self.allocator, path, max_password_bytes);
    }

    pub fn deinit(self: *Guardian) void {
        self.nodes.deinit();
        // Zero out secrets
        @memset(&self.server_secret, 0);
        @memset(&self.integrity_key, 0);
    }

    /// Get current write quorum requirement. The caller holds nodes_mu.
    pub fn writeQuorum(self: *const Guardian) usize {
        return quorum.writeQuorum(self.nodes.aliveCount());
    }

    /// Get current Shamir threshold (read quorum).
    ///
    /// This used to call a second implementation on NodeList that still
    /// returned max(3, floor(N/3)) after quorum.zig moved to a floor of 2, so
    /// GET /v1/vault/guardians advertised a threshold the cluster does not use.
    /// quorum.readQuorum is the one definition. The caller holds nodes_mu.
    pub fn readThreshold(self: *const Guardian) usize {
        return quorum.readQuorum(self.nodes.aliveCount());
    }

    /// Refresh share count from disk.
    pub fn refreshShareCount(self: *Guardian) void {
        self.share_count = heartbeat.countShares(self.cfg.data_dir);
    }
};

/// Load the persistent at-rest integrity key from <data_dir>/integrity.key,
/// creating it with fresh randomness on first run.
///
/// This key authenticates share files on disk, so it MUST persist across
/// restarts. The previous implementation keyed at-rest integrity with the
/// ephemeral server_secret, which meant every stored share became unreadable
/// after any restart (deploy, crash, reboot). If the key cannot be persisted we
/// fall back to an in-memory random key and warn loudly (degraded: shares
/// written this boot won't survive a restart).
fn loadOrCreateIntegrityKey(data_dir: []const u8, allocator: std.mem.Allocator) [32]u8 {
    var key: [32]u8 = undefined;

    const path = std.fmt.allocPrint(allocator, "{s}/integrity.key", .{data_dir}) catch {
        std.crypto.random.bytes(&key);
        log.err("integrity key path alloc failed; using ephemeral key (shares will not survive restart)", .{});
        return key;
    };
    defer allocator.free(path);

    // Load an existing, well-formed key if present.
    if (std.fs.cwd().readFileAlloc(allocator, path, 32)) |existing| {
        defer allocator.free(existing);
        if (existing.len == 32) {
            @memcpy(&key, existing[0..32]);
            return key;
        }
        log.warn("integrity.key has unexpected size {d}; regenerating", .{existing.len});
    } else |_| {}

    // Create a fresh key and persist it atomically with 0600 perms.
    std.crypto.random.bytes(&key);
    std.fs.cwd().makePath(data_dir) catch {};

    const tmp_path = std.fmt.allocPrint(allocator, "{s}/integrity.key.tmp", .{data_dir}) catch {
        log.err("integrity key persist failed (tmp path); using ephemeral key", .{});
        return key;
    };
    defer allocator.free(tmp_path);

    persist: {
        const file = std.fs.cwd().createFile(tmp_path, .{ .mode = 0o600 }) catch {
            log.err("could not create integrity key file; using ephemeral key (shares will not survive restart)", .{});
            break :persist;
        };
        {
            defer file.close();
            file.writeAll(&key) catch {
                log.err("could not write integrity key; using ephemeral key", .{});
                break :persist;
            };
        }
        std.fs.cwd().rename(tmp_path, path) catch {
            std.fs.cwd().deleteFile(tmp_path) catch {};
            log.err("could not finalize integrity key; using ephemeral key", .{});
            break :persist;
        };
        log.info("initialized persistent vault integrity key", .{});
    }
    return key;
}

// ── Tests ────────────────────────────────────────────────────────────────────

test "guardian: init and deinit" {
    const allocator = std.testing.allocator;
    const cfg = config.Config{
        .data_dir = "/tmp/nonexistent-vault-test",
    };

    var g = try Guardian.init(allocator, cfg);
    defer g.deinit();

    try std.testing.expectEqual(@as(u32, 0), g.share_count);
}
