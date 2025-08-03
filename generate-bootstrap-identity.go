package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	// Generate a fixed identity
	priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		panic(err)
	}

	// Get peer ID
	peerID, err := peer.IDFromPublicKey(pub)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Generated Peer ID: %s\n", peerID.String())

	// Marshal private key
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		panic(err)
	}

	// Create data directory
	dataDir := "./data/bootstrap"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		panic(err)
	}

	// Save identity
	identityFile := filepath.Join(dataDir, "identity.key")
	if err := os.WriteFile(identityFile, data, 0600); err != nil {
		panic(err)
	}

	fmt.Printf("Identity saved to: %s\n", identityFile)
	fmt.Printf("Bootstrap address: /ip4/127.0.0.1/tcp/4001/p2p/%s\n", peerID.String())
}
