package gateway

import (
	"fmt"
	"path/filepath"

	"github.com/Orama-Network/orama/core/pkg/attest"
	"github.com/Orama-Network/orama/core/pkg/attest/android"
	"github.com/Orama-Network/orama/core/pkg/attest/apple"
	"github.com/Orama-Network/orama/core/pkg/attest/graphene"
	"github.com/Orama-Network/orama/core/pkg/attest/revocation"
	attesthandlers "github.com/Orama-Network/orama/core/pkg/gateway/handlers/attest"
	"github.com/Orama-Network/orama/core/pkg/telemetry/report"
	"go.uber.org/zap"
)

// attestationCacheDir is where the index gateway keeps the last good copy of
// Google's attestation revocation list and GrapheneOS's key list, under the
// node's data directory, so a restart verifies Android devices at once rather
// than after its first fetch.
const attestationCacheDir = "attest"

// attestationLists are the two lists Android verification reads, refreshed in
// the background (core/pkg/attest/revocation, core/pkg/attest/graphene).
type attestationLists struct {
	revocation *revocation.Refresher
	graphene   *graphene.Refresher
}

// wireAttestation gives the auth service its attestation verifier and builds
// the attestation handlers.
//
// The attestation routes are served by the index gateway alone (MainGateway),
// so only it fetches the two lists: once a day per node, whatever the number
// of namespaces. A namespace gateway verifies only iOS assertions, at a
// session refresh; its Android verifier holds lists that are never fetched,
// and would answer a server error rather than a verdict if it were ever asked.
func (g *Gateway) wireAttestation() error {
	logger := g.logger.Logger
	revCfg := revocation.Config{Logger: logger}
	gosCfg := graphene.Config{Logger: logger}
	index := !isNamespaceGateway(g.cfg)
	if index && g.cfg.DataDir != "" {
		dir := filepath.Join(g.cfg.DataDir, attestationCacheDir)
		revCfg.CacheFile = filepath.Join(dir, "android-revocations.json")
		gosCfg.CacheFile = filepath.Join(dir, "grapheneos-keys.json")
	}
	rev, err := revocation.New(revCfg)
	if err != nil {
		return fmt.Errorf("build the Android attestation revocation list: %w", err)
	}
	gos, err := graphene.New(gosCfg)
	if err != nil {
		return fmt.Errorf("build the GrapheneOS key list: %w", err)
	}
	if index {
		go rev.Run(g.shutdownCtx)
		go gos.Run(g.shutdownCtx)
		g.attestationLists = &attestationLists{revocation: rev, graphene: gos}
	}
	g.authService.SetAttestationVerifier(attest.New(apple.New(), android.New(rev, gos)))
	g.attestHandlers = attesthandlers.NewHandlers(g.authService, g.authService.Audit(), g.logger)
	logger.Debug("device attestation wired", zap.Bool("fetches_lists", index))
	return nil
}

// attestationListsReport is the state of the two lists for the node report:
// when each was fetched and whether it is stale. Nil on a gateway that fetches
// none.
func (g *Gateway) attestationListsReport() *report.AttestationListsReport {
	if g.attestationLists == nil {
		return nil
	}
	rev, gos := g.attestationLists.revocation.Status(), g.attestationLists.graphene.Status()
	return &report.AttestationListsReport{
		RevocationFetchedAt: rev.FetchedAt,
		RevocationStale:     g.attestationLists.revocation.Stale(),
		GrapheneFetchedAt:   gos.FetchedAt,
		GrapheneStale:       g.attestationLists.graphene.Stale(),
	}
}
