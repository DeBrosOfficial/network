package setup

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The pins in create_genesis.go and the values oramad refuses a production genesis for must be one
// number. The chain's side is read from its source, which sits beside core/.
func TestConsensusConstants_equalTheChainsPins(t *testing.T) {
	data, err := os.ReadFile("../../../../../chain/app/consensus_params.go")
	if err != nil {
		t.Skipf("the chain sources are not reachable from here: %v", err)
	}
	src := string(data)
	for name, want := range map[string]string{
		"ConsensusMaxGas":                      consensusMaxGas,
		"ConsensusMaxBlockBytes":               consensusMaxBlockBytes,
		"ConsensusVoteExtensionsEnableHeight":  voteExtensionsEnableHeight,
		"ConsensusEvidenceMaxAgeNumBlocks":     evidenceMaxAgeNumBlocks,
		"ConsensusEvidenceMaxAgeDurationNanos": evidenceMaxAgeDurationNanos,
		"ConsensusEvidenceMaxBytes":            evidenceMaxBytes,
	} {
		m := regexp.MustCompile(name + `\s+int64 = ([0-9_ *]+)\n`).FindStringSubmatch(src)
		if m == nil {
			t.Fatalf("chain/app/consensus_params.go no longer declares %s: update setup/create_genesis.go", name)
		}
		got := int64(1)
		for _, f := range strings.Split(strings.ReplaceAll(m[1], "_", ""), "*") {
			n, err := strconv.ParseInt(strings.TrimSpace(f), 10, 64)
			if err != nil {
				t.Fatalf("%s = %q: %v", name, m[1], err)
			}
			got *= n
		}
		if strconv.FormatInt(got, 10) != want {
			t.Errorf("%s is %d in the chain and %q in setup", name, got, want)
		}
	}
	if !regexp.MustCompile(`ConsensusValidatorKeyType = "` + validatorKeyType + `"`).MatchString(src) {
		t.Errorf("chain/app/consensus_params.go no longer pins the validator key type %q: update setup/create_genesis.go", validatorKeyType)
	}
}

func TestApplyConsensusParams_writesTheBlockSizeEvenWhenOramadLeftAnother(t *testing.T) {
	genesis := `{"chain_id":"orama-1","consensus":{"params":{"block":{"max_bytes":"1048576","max_gas":"-1"}}}}`
	out, err := ApplyConsensusParams([]byte(genesis))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeParams(t, out)["block"]["max_bytes"]; got != consensusMaxBlockBytes {
		t.Errorf("max_bytes = %v, want %s", got, consensusMaxBlockBytes)
	}
}

func TestVerifyGenesis_aBlockSizeOtherThanThePinIsRefused(t *testing.T) {
	doc, err := ApplyConsensusParams(genesisFor{chainID: "orama-stagenet-6", seats: fakeSeats(2), allowStake: true, faucet: true}.json())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyGenesis(doc, verifySpec(2)); err != nil {
		t.Fatalf("the genesis setup writes was refused: %v", err)
	}
	tampered := []byte(strings.Replace(string(doc), `"max_bytes": "`+consensusMaxBlockBytes+`"`, `"max_bytes": "-1"`, 1))
	if err := VerifyGenesis(tampered, verifySpec(2)); err == nil || !strings.Contains(err.Error(), "max_bytes") {
		t.Errorf("an unlimited block size was accepted: %v", err)
	}
}

func TestVerifyGenesis_aValidatorKeyTypeOrEvidenceWindowOtherThanThePinIsRefused(t *testing.T) {
	doc, err := ApplyConsensusParams(genesisFor{chainID: "orama-stagenet-6", seats: fakeSeats(2), allowStake: true, faucet: true}.json())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyGenesis(doc, verifySpec(2)); err != nil {
		t.Fatalf("the genesis setup writes was refused: %v", err)
	}
	for name, tamper := range map[string]struct{ from, to, want string }{
		"a second key type":       {`"ed25519"`, `"ed25519", "secp256k1"`, "pub_key_types"},
		"a shorter evidence age":  {`"max_age_num_blocks": "` + evidenceMaxAgeNumBlocks + `"`, `"max_age_num_blocks": "10"`, "evidence"},
		"a shorter evidence time": {`"max_age_duration": "` + evidenceMaxAgeDurationNanos + `"`, `"max_age_duration": "60000000000"`, "evidence"},
	} {
		bad := []byte(strings.Replace(string(doc), tamper.from, tamper.to, 1))
		if string(bad) == string(doc) {
			t.Fatalf("%s: the genesis has no %s to tamper with:\n%s", name, tamper.from, doc)
		}
		if err := VerifyGenesis(bad, verifySpec(2)); err == nil || !strings.Contains(err.Error(), tamper.want) {
			t.Errorf("%s was accepted: %v", name, err)
		}
	}
}
