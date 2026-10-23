package client

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Orama-Network/orama/core/pkg/pubsub"
)

// rqlite read consistency levels a client can be configured with.
const (
	// ReadLevelNone reads this node's local replica: fastest, but it can miss
	// a write the leader has acknowledged and this node has not yet applied.
	ReadLevelNone = "none"
	// ReadLevelWeak routes the read to the leader, which has every
	// acknowledged write. It costs one hop when this node is not the leader.
	ReadLevelWeak = "weak"
)

// ClientConfig represents configuration for network clients
type ClientConfig struct {
	AppName        string   `json:"app_name"`
	DatabaseName   string   `json:"database_name"`
	BootstrapPeers []string `json:"peers"`
	// DatabaseEndpoints are RQLite nodes the database client dials directly,
	// which needs a route into the WireGuard mesh: it is how a gateway reaches
	// its own database. Left empty, the database client goes through the
	// gateway at GatewayURL with the client's credential. DefaultDatabaseEndpoints
	// is the in-mesh default.
	DatabaseEndpoints []string `json:"database_endpoints"`
	// DatabaseReadLevel is the rqlite consistency level reads are served at:
	// ReadLevelNone (the default when empty) or ReadLevelWeak.
	DatabaseReadLevel string        `json:"database_read_level"`
	GatewayURL        string        `json:"gateway_url"` // Gateway URL for HTTP API access
	ConnectTimeout    time.Duration `json:"connect_timeout"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration `json:"retry_delay"`
	QuietMode         bool          `json:"quiet_mode"`    // Suppress debug/info logs
	APIKey            string        `json:"api_key"`       // API key for gateway auth
	JWT               string        `json:"jwt"`           // Optional JWT bearer token
	IdentityPath      string        `json:"identity_path"` // Path to persistent LibP2P identity key file
	PubSubSocket      string        `json:"pubsub_socket"` // unix socket of the node's app GossipSub API (orama-namespace-pubsub@index)

	// IPFSClusterAPIPassword authenticates the network status's read of this
	// node's IPFS Cluster REST API (ipfs.ClusterRESTPassword). Never
	// serialised.
	IPFSClusterAPIPassword string `json:"-"`

	// ListenAddrs are the libp2p multiaddrs the client's host accepts
	// connections on, e.g. "/ip4/10.0.0.3/tcp/0" for a node's WireGuard
	// address. Empty means no listener: the client only dials its bootstrap
	// peers, which needs none. An unspecified address (0.0.0.0, ::) is
	// refused — it is every interface, the public one included.
	ListenAddrs []string `json:"listen_addrs"`

	// HTTPClient carries every request to GatewayURL: storage, and the
	// database when there are no DatabaseEndpoints. Its Transport (TLS
	// configuration, root CAs, proxy) and Jar are used; the per-operation
	// timeout and the refusal to follow redirects are the client's own and
	// replace its Timeout and CheckRedirect. Nil uses http.DefaultTransport,
	// which trusts the system roots only, so a gateway behind a private or
	// staging CA needs a Transport whose TLSClientConfig.RootCAs holds that
	// CA. Never serialised.
	HTTPClient *http.Client `json:"-"`
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig(appName string) *ClientConfig {
	// Base defaults
	peers := DefaultBootstrapPeers()

	return &ClientConfig{
		AppName:        appName,
		DatabaseName:   fmt.Sprintf("%s_db", appName),
		BootstrapPeers: peers,
		GatewayURL:     "",
		ConnectTimeout: time.Second * 30,
		RetryAttempts:  3,
		RetryDelay:     time.Second * 5,
		QuietMode:      false,
		APIKey:         "",
		JWT:            "",
		PubSubSocket:   pubsub.DefaultSocketPath,
	}
}
