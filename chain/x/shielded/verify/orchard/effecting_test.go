package orchard

import "github.com/Orama-Network/orama/chain/x/shielded/bundle"

// effectingData is the bundle prefix the signatures cover (bundle.EffectingData).
func effectingData(b []byte) ([]byte, error) { return bundle.EffectingData(b) }
