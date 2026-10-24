package gateway

import "testing"

// API keys are stored HMAC-hashed and the plaintext migration moves every
// raw-key principal to the hashed form, so a key is looked up under its hash
// alone: the raw key must never be a candidate.

func TestApiKeyOwnerCandidates_apiKeyChecksOnlyTheHash(t *testing.T) {
	got := principalIdentifierCandidates("api_key", "ak_raw", "HASHED")
	if len(got) != 1 || got[0] != "HASHED" {
		t.Errorf("api_key: want [HASHED] only; got %v", got)
	}
}

// A wallet is never hashed. It may be case-normalised (bug-329), so assert the
// property that matters rather than the candidate count: the hashed form of the
// key must never appear among the values checked for a wallet owner.
func TestApiKeyOwnerCandidates_walletIsNeverHashed(t *testing.T) {
	got := principalIdentifierCandidates("wallet", "0xWALLET", "HASHED")
	if len(got) == 0 {
		t.Fatal("wallet must yield at least one candidate")
	}
	for _, c := range got {
		if c == "HASHED" {
			t.Errorf("wallet candidates must never include the hashed form; got %v", got)
		}
	}
	if got[len(got)-1] != "0xWALLET" {
		t.Errorf("the presented spelling must remain a candidate; got %v", got)
	}
}

// A wallet stored before normalisation kept whatever case the client sent.
// Ownership lookups must therefore try the normalised spelling first and the
// presented one second, so those rows keep working until migration 042 has run.
func TestOwnerCandidates_WalletChecksTheNormalisedFormFirst(t *testing.T) {
	got := principalIdentifierCandidates("wallet", "0xAbCdEf", "")
	want := []string{"0xabcdef", "0xAbCdEf"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// An already-lowercase wallet needs only one candidate.
func TestOwnerCandidates_LowercaseWalletIsASingleCandidate(t *testing.T) {
	got := principalIdentifierCandidates("wallet", "0xabcdef", "")
	if len(got) != 1 || got[0] != "0xabcdef" {
		t.Fatalf("got %v, want a single normalised candidate", got)
	}
}
