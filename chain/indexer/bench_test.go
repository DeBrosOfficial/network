package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"cosmossdk.io/math"
	gogoproto "github.com/cosmos/gogoproto/proto"

	emissiontypes "github.com/DeBrosOfficial/network/chain/x/emission/types"
	feestypes "github.com/DeBrosOfficial/network/chain/x/fees/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/DeBrosOfficial/network/chain/app/params"
	"github.com/cockroachdb/pebble"
)

// The load test builds an index the way the follower leaves one, without a chain to follow, and reads
// it through the HTTP API. It is a benchmark, so `go test` skips it:
//
//	go test -run '^$' -bench BenchmarkIndexAtScale -benchtime 300x ./indexer/
//
// ORAMA_INDEX_BENCH_TXS sets the number of transactions (default one million).
const (
	benchDefaultTxs   = 1_000_000
	benchTxsPerBlock  = 10
	benchAccounts     = 20_000
	benchValidators   = 100
	benchEpochs       = 500
	benchBlocksPerEp  = 200
	benchBatchBlocks  = 1000
	benchBlockSeconds = 2
	benchFailedEvery  = 50
)

// benchChain is the node the home snapshot reads: a tip and two queries.
type benchChain struct{ tip int64 }

func (c benchChain) HeightRange(context.Context) (int64, int64, error) { return 1, c.tip, nil }

func (c benchChain) QueryAt(_ context.Context, _ int64, method string, _, resp gogoproto.Message) error {
	switch r := resp.(type) {
	case *banktypes.QuerySupplyOfResponse:
		r.Amount = sdkCoin(1_000_000_000)
	case *feestypes.QueryBaseFeeResponse:
		r.BaseFee = math.NewInt(7)
	case *emissiontypes.QueryCurrentEpochResponse:
		r.EpochState = emissiontypes.EpochState{CurrentEpoch: benchEpochs + 1, EpochStartUnixNano: time.Now().UnixNano()}
	case *emissiontypes.QueryParamsResponse:
		r.Params = emissiontypes.Params{EpochDurationSeconds: benchBlocksPerEp * benchBlockSeconds}
	default:
		return fmt.Errorf("the bench chain answers no %s", method)
	}
	return nil
}

// synthetic is a built index and the keys the benchmarks read it by.
type synthetic struct {
	store      *Store
	txs        int
	blocks     int64
	busy       string
	firstHash  string
	validator  string
	epochs     int
	buildTime  time.Duration
	onDiskSize int64
}

func benchAddr(prefix string, n int) string {
	raw := make([]byte, 20)
	binary.BigEndian.PutUint32(raw[16:], uint32(n))
	s, err := bech32.ConvertAndEncode(prefix, raw)
	if err != nil {
		panic(err)
	}
	return s
}

func benchTxs(b *testing.B) int {
	b.Helper()
	if raw := os.Getenv("ORAMA_INDEX_BENCH_TXS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < benchTxsPerBlock {
			b.Fatalf("ORAMA_INDEX_BENCH_TXS=%q is not a number of at least %d", raw, benchTxsPerBlock)
		}
		return n
	}
	return benchDefaultTxs
}

// messageFor picks a transaction's message type: 60% transfers, 20% staking, 5% storage, the rest none.
func messageFor(r *rand.Rand) string {
	switch n := r.Intn(100); {
	case n < 60:
		return "/cosmos.bank.v1beta1.MsgSend"
	case n < 80:
		return "/cosmos.staking.v1beta1.MsgDelegate"
	case n < 85:
		return "/orama.storage.v1.MsgCreateDeal"
	default:
		return "/orama.cnft.v1.MsgMint"
	}
}

func buildSynthetic(b *testing.B, dir string, txs int) synthetic {
	b.Helper()
	start := time.Now()
	store, err := Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	r := rand.New(rand.NewSource(1))
	accounts := make([]string, benchAccounts)
	for i := range accounts {
		accounts[i] = benchAddr(params.Bech32Prefix, i+1)
	}
	out := synthetic{store: store, txs: txs, busy: accounts[0]}
	blocks := int64(txs / benchTxsPerBlock)
	origin := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := &writer{b: store.db.NewBatch()}
	counts := map[string]*hourRecord{}
	summaries := map[string]*AccountSummary{}
	seq := 0
	for h := int64(1); h <= blocks; h++ {
		bt := origin.Add(time.Duration(h*benchBlockSeconds) * time.Second)
		block := Block{Height: h, Hash: hexOf(uint64(h)), Time: bt, Proposer: hexOf(uint64(h % benchValidators)), Burned: "0", CommitSigned: benchValidators - 1, CommitSize: benchValidators}
		for i := 0; i < benchTxsPerBlock; i++ {
			seq++
			failed := seq%benchFailedEvery == 0
			a := accounts[r.Intn(benchAccounts)]
			if r.Intn(10) == 0 {
				a = out.busy
			}
			hash := sha256.Sum256([]byte(strconv.Itoa(seq)))
			tx := syntheticTx(hash[:], h, uint32(i), bt, a, accounts[r.Intn(benchAccounts)], messageFor(r), failed)
			mustBench(b, indexSyntheticTx(batch, tx, hash[:], counts, summaries))
			block.TxHashes = append(block.TxHashes, tx.Hash)
			if seq == 1 {
				out.firstHash = tx.Hash
			}
		}
		block.TxCount = len(block.TxHashes)
		mustBench(b, batch.putBlock(block))
		if h%benchBatchBlocks == 0 || h == blocks {
			mustBench(b, batch.b.Commit(pebble.NoSync))
			batch = &writer{b: store.db.NewBatch()}
		}
	}
	mustBench(b, writeBuckets(batch, counts))
	for addr, sum := range summaries {
		mustBench(b, batch.setJSON(summaryKey(addr), sum))
	}
	out.blocks = blocks
	out.validator, out.epochs = writeEpochs(b, batch, blocks, origin)
	mustBench(b, batch.set([]byte(keyStart), be64(1)))
	mustBench(b, batch.set([]byte(keyCursor), be64(uint64(blocks))))
	mustBench(b, batch.set([]byte(keyAggFrom), be64(1)))
	mustBench(b, batch.b.Commit(pebble.Sync))
	mustBench(b, store.db.Flush())
	out.buildTime = time.Since(start)
	out.onDiskSize = dirSize(dir)
	return out
}

func syntheticTx(hash []byte, height int64, index uint32, bt time.Time, from, to, msg string, failed bool) Tx {
	tx := Tx{
		Hash: hex.EncodeToString(hash), Height: height, Index: index, Time: bt, GasWanted: 200000, GasUsed: 81234,
		Messages: []string{msg}, Signer: from, Memo: "load test",
		Events: []Event{
			{Type: "tx", Attributes: []Attribute{{Key: "fee", Value: "2000norama"}, {Key: "base_fee", Value: "1000"}}},
			{Type: "transfer", Attributes: []Attribute{{Key: "sender", Value: from}, {Key: "recipient", Value: to}, {Key: "amount", Value: "12norama"}}},
		},
		Body: []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"@type":%q,"from_address":%q,"to_address":%q,"amount":[{"denom":"norama","amount":"12"}]}`, msg, from, to))},
	}
	if failed {
		tx.Code, tx.Codespace, tx.Log = 5, "sdk", "insufficient funds"
	}
	return tx
}

// indexSyntheticTx writes what the follower writes for one transaction: the row, the latest list, the
// filtered lists, and the account lists of the two accounts its events name.
func indexSyntheticTx(w *writer, tx Tx, hash []byte, counts map[string]*hourRecord, summaries map[string]*AccountSummary) error {
	kinds := kindsOf(tx.Messages, tx.Code != 0)
	if err := w.setJSON(txKey(hash), tx); err != nil {
		return err
	}
	if err := w.addLatest(tx.Height, tx.Index, hash); err != nil {
		return err
	}
	if err := w.addKinds(kinds, tx.Height, tx.Index, hash); err != nil {
		return err
	}
	for _, key := range [][]byte{hourKey(hourOf(tx.Time)), dayKey(dayOf(tx.Time)), weekKey(weekOf(tx.Time))} {
		rec := counts[string(key)]
		if rec == nil {
			rec = &hourRecord{Burned: "0"}
			counts[string(key)] = rec
		}
		rec.Txs++
		if tx.Code != 0 {
			rec.Failed++
		}
	}
	for _, addr := range []string{tx.Signer, tx.Events[1].Attributes[1].Value} {
		if err := w.addAccountTx(addr, tx.Height, tx.Index, hash); err != nil {
			return err
		}
		if err := w.addAccountKinds(addr, kinds, tx.Height, tx.Index, hash); err != nil {
			return err
		}
		sum := summaries[addr]
		if sum == nil {
			sum = &AccountSummary{Address: addr, FirstSeen: tx.Time}
			summaries[addr] = sum
		}
		sum.TxCount++
		sum.LastActive = tx.Time
	}
	return nil
}

func writeBuckets(w *writer, counts map[string]*hourRecord) error {
	for key, rec := range counts {
		if err := w.setJSON([]byte(key), rec); err != nil {
			return err
		}
	}
	return nil
}

// writeEpochs stores closed epochs, each with a row per validator and a reward for each, and the
// registry, as the follower leaves them. It returns the operator of one validator.
func writeEpochs(b *testing.B, w *writer, blocks int64, origin time.Time) (string, int) {
	b.Helper()
	ops := make([]string, benchValidators)
	cons := make([][]byte, benchValidators)
	for i := range ops {
		ops[i] = benchAddr(params.Bech32PrefixValAddr, i+1)
		cons[i] = make([]byte, consLen)
		binary.BigEndian.PutUint32(cons[i][16:], uint32(i+1))
		consStr, err := consBech32(cons[i])
		mustBench(b, err)
		mustBench(b, w.putValidator(cons[i], ValidatorInfo{Operator: ops[i], ConsensusAddress: consStr, Moniker: "validator", Status: StatusBonded, Tokens: "1000", CometPower: int64(1000 - i)}))
	}
	epochs := min(benchEpochs, int(blocks/benchBlocksPerEp))
	for e := uint64(1); e <= uint64(epochs); e++ {
		start := int64(e-1)*benchBlocksPerEp + 1
		row := EpochRow{Epoch: e, StartHeight: start, EndHeight: start + benchBlocksPerEp - 1, Blocks: benchBlocksPerEp,
			StartTime: origin.Add(time.Duration(start*benchBlockSeconds) * time.Second), EndTime: origin.Add(time.Duration((start+benchBlocksPerEp)*benchBlockSeconds) * time.Second),
			MintedValidators: "600000", RewardsRecorded: true, RewardsCommission: "1000", RewardsOperatorShare: "300000", RewardsDelegators: "300000"}
		mustBench(b, w.setJSON(epochKey(e), row))
		for i := range ops {
			ve := ValidatorEpoch{Epoch: e, Operator: ops[i], CometPower: int64(1000 - i), SignedBlocks: 190, MissedBlocks: 10, UptimeBps: 9500, SlashedBurned: "0", Status: StatusBonded}
			mustBench(b, w.putValidatorEpoch(e, cons[i], ValidatorInfo{Operator: ops[i], CometPower: ve.CometPower, Status: StatusBonded}, counters{Signed: 190, Missed: 10, Burned: math.ZeroInt()}))
			rw := ValidatorReward{Epoch: e, Operator: ops[i], Amount: "6000", Commission: "10", OperatorShare: "3000", Delegators: "3000", ForceBonded: "0"}
			mustBench(b, w.setJSON(rewardEpochKey(e, ops[i]), rw))
			mustBench(b, w.setJSON(rewardValKey(ops[i], e), rw))
		}
	}
	mustBench(b, w.set([]byte(keyRewardsFrom), be64(1)))
	mustBench(b, w.putWatch(watch{Epoch: uint64(epochs) + 1, StartUnixNano: origin.UnixNano(), DurationSec: benchBlocksPerEp * benchBlockSeconds}))
	return ops[0], epochs
}

func hexOf(n uint64) string {
	return hex.EncodeToString(binary.BigEndian.AppendUint64(make([]byte, 24), n))
}

func mustBench(b *testing.B, err error) {
	b.Helper()
	if err != nil {
		b.Fatal(err)
	}
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// BenchmarkIndexAtScale reads a large index through the API: the routes the explorer calls for a
// page, each as the HTTP handler answers it (the JSON is encoded; the network is not in it).
func BenchmarkIndexAtScale(b *testing.B) {
	txs := benchTxs(b)
	idx := buildSynthetic(b, filepath.Join(b.TempDir(), "index"), txs)
	defer idx.store.Close()
	b.Logf("index of %d transactions in %d blocks, %d accounts, %d epochs of %d validators: built in %s, %.0f MB on disk",
		idx.txs, idx.blocks, benchAccounts, idx.epochs, benchValidators, idx.buildTime.Round(time.Second), float64(idx.onDiskSize)/(1<<20))
	api := NewAPI(idx.store, benchChain{tip: idx.blocks})
	deepest := fmt.Sprintf("page=%d&limit=%d", MaxPage, MaxLimit)
	lastEpoch := idx.epochs

	routes := []struct{ name, target string }{
		{"status", "/index/v1/status"},
		{"txs_latest_20", "/index/v1/txs?limit=20"},
		{"txs_page1000_limit100", "/index/v1/txs?" + deepest},
		{"txs_failed_page1", "/index/v1/txs?filter=failed&limit=20"},
		{"txs_transfers_page1000", "/index/v1/txs?filter=transfers&" + deepest},
		{"txs_storage_page1", "/index/v1/txs?filter=storage&limit=20"},
		{"account_txs_busy_page1", "/index/v1/accounts/" + idx.busy + "/txs?limit=20"},
		{"account_txs_busy_staking", "/index/v1/accounts/" + idx.busy + "/txs?filter=staking&limit=20"},
		{"account_summary", "/index/v1/accounts/" + idx.busy},
		{"tx_by_hash", "/index/v1/txs/" + idx.firstHash},
		{"blocks_page1_limit24", "/index/v1/blocks?limit=24"},
		{"blocks_page1000_limit100", "/index/v1/blocks?" + deepest},
		{"block_by_height", "/index/v1/blocks/" + strconv.FormatInt(idx.blocks/2, 10)},
		{"stats_hourly_48h", "/index/v1/stats"},
		{"stats_daily", "/index/v1/stats/daily?limit=30"},
		{"epochs_page1_limit100", "/index/v1/epochs?limit=100"},
		{"epoch_rewards_100_validators", "/index/v1/epochs/" + strconv.Itoa(lastEpoch) + "/rewards?limit=100"},
		{"validator_rewards_page1", "/index/v1/validators/" + idx.validator + "/rewards?limit=20"},
		{"account_rewards", "/index/v1/accounts/" + idx.busy + "/rewards"},
	}
	for _, rt := range routes {
		b.Run(rt.name, func(b *testing.B) { benchRoute(b, api, rt.target) })
	}
	b.Run("home_cached", func(b *testing.B) { benchRoute(b, api, "/index/v1/home") })
	b.Run("home_rebuilt", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			api.home = homeCache{}
			benchServe(b, api, "/index/v1/home")
		}
	})
}

func benchRoute(b *testing.B, api *API, target string) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchServe(b, api, target)
	}
}

func benchServe(b *testing.B, api *API, target string) {
	b.Helper()
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if rec.Code != http.StatusOK {
		b.Fatalf("%s answered %d: %s", target, rec.Code, rec.Body.String())
	}
	b.SetBytes(int64(rec.Body.Len()))
}
