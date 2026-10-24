package hostfunctions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/pubsub"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	"github.com/Orama-Network/orama/core/pkg/serverless"
	"github.com/Orama-Network/orama/core/pkg/sqlguard"
	"go.uber.org/zap"
)

// refusingDB is a database handle that panics if anything reaches it. The
// guard's job is that nothing does.
type refusingDB struct{ rqlite.Client }

// Every DB host function has to ask. One that does not is the whole hole
// reopened through a different name.
func TestDBHostFunctions_allRefuseAProtectedTable(t *testing.T) {
	h := &HostFunctions{logger: zap.NewNop(), db: refusingDB{}, dbNamespace: testNamespace}
	// Inside an invocation of the database's own namespace, so the refusal
	// that comes back is the guard's and not the namespace check's.
	ctx := nsCtx()
	protected := "SELECT * FROM api_keys"
	ops := []byte(`{"ops":[{"kind":"exec","sql":"SELECT * FROM api_keys"}]}`)

	calls := map[string]func() error{
		"db_query":       func() error { _, err := h.DBQuery(ctx, protected, nil); return err },
		"db_execute":     func() error { _, err := h.DBExecute(ctx, protected, nil); return err },
		"db_execute_v2":  func() error { _, err := h.DBExecuteV2(ctx, protected, nil); return err },
		"db_query_v2":    func() error { _, err := h.DBQueryV2(ctx, protected, nil); return err },
		"db_transaction": func() error { _, err := h.DBTransaction(ctx, ops); return err },
		"db_query_batch": func() error {
			_, err := h.DBQueryBatch(ctx, []byte(`{"ops":[{"sql":"SELECT * FROM api_keys"}]}`))
			return err
		},
	}
	for fn, call := range calls {
		var refused *sqlguard.ErrNotAllowed
		if err := call(); !errors.As(err, &refused) {
			t.Errorf("%s: err = %v, want the SQL guard's refusal", fn, err)
		}
	}
	pubCtx := invocationCtx(&serverless.InvocationContext{Namespace: testNamespace})
	withBus := &HostFunctions{logger: zap.NewNop(), db: refusingDB{}, dbNamespace: testNamespace, pubsub: &publishingBus{}}
	if _, err := withBus.ExecAndPublish(pubCtx, []byte(`{"ops":[{"kind":"exec","sql":"SELECT * FROM api_keys"}]}`), "wake", []byte("{}")); err == nil {
		t.Error("exec_and_publish ran it")
	}
}

// publishingBus is a pubsub bus that only counts publishes; exec_and_publish
// refuses to run without one, so the guard test needs it to get that far.
type publishingBus struct {
	pubsub.Bus
	published int
}

func (b *publishingBus) Publish(context.Context, string, []byte) error {
	b.published++
	return nil
}

// bugboard #425: exec_and_publish ran the guest's ops with no guard at all, so
// it reached the auth tables every other database host function refuses. The
// refusal comes before the batch: refusingDB has no BatchWithSeq, so reaching
// it would panic.
func TestExecAndPublish_refusesAProtectedTable(t *testing.T) {
	bus := &publishingBus{}
	h := &HostFunctions{logger: zap.NewNop(), db: refusingDB{}, dbNamespace: testNamespace, pubsub: bus}
	ctx := serverless.WithPublishCounter(invocationCtx(&serverless.InvocationContext{Namespace: testNamespace}))

	for _, ops := range []string{
		`{"ops":[{"kind":"exec","sql":"UPDATE api_keys SET scopes='admin'"}]}`,
		`{"ops":[{"kind":"exec","sql":"INSERT INTO messages(body) VALUES ('x')"},{"kind":"query","sql":"SELECT * FROM function_secrets"}]}`,
		`{"ops":[{"kind":"exec","sql":"INSERT INTO messages(body) VALUES ('x'); DELETE FROM grants"}]}`,
	} {
		_, err := h.ExecAndPublish(ctx, []byte(ops), "wake", []byte("{}"))
		var refused *sqlguard.ErrNotAllowed
		if !errors.As(err, &refused) {
			t.Errorf("exec_and_publish(%s): err = %v, want the SQL guard's refusal", ops, err)
		}
	}
	if bus.published != 0 {
		t.Errorf("a refused call published %d wake-ups", bus.published)
	}
	// Adding one to the counter reads back 1 only if no refused call charged
	// the invocation's publish budget.
	if n := serverless.AddPublishCount(ctx, 1); n != 1 {
		t.Errorf("refused calls charged the publish budget: counter at %d after one more publish", n)
	}
}

// A function is a tenant's code: it may not write a trigger row that makes the
// platform run a function without a caller check, or a history row that picks
// a rollback's CID. Every database host function refuses, before the database.
func TestDBHostFunctions_allRefuseTriggerAndHistoryTables(t *testing.T) {
	h := &HostFunctions{logger: zap.NewNop(), db: refusingDB{}, dbNamespace: testNamespace, pubsub: &publishingBus{}}
	ctx := nsCtx()
	pubCtx := serverless.WithPublishCounter(invocationCtx(&serverless.InvocationContext{Namespace: testNamespace}))

	for _, sql := range []string{
		"INSERT INTO function_cron_triggers (id, function_id, cron_expression) VALUES ('t', 'f', '* * * * *')",
		"INSERT INTO function_pubsub_triggers (id, function_id, topic) VALUES ('t', 'f', 'x')",
		"INSERT INTO deployment_history (id, deployment_id, content_cid) VALUES ('h', 'd', 'QmForged')",
		"UPDATE functions SET is_public = 1",
	} {
		opsJSON, _ := json.Marshal(map[string]any{"ops": []map[string]any{{"kind": "exec", "sql": sql}}})
		calls := map[string]func() error{
			"db_query":       func() error { _, err := h.DBQuery(ctx, sql, nil); return err },
			"db_execute":     func() error { _, err := h.DBExecute(ctx, sql, nil); return err },
			"db_execute_v2":  func() error { _, err := h.DBExecuteV2(ctx, sql, nil); return err },
			"db_query_v2":    func() error { _, err := h.DBQueryV2(ctx, sql, nil); return err },
			"db_transaction": func() error { _, err := h.DBTransaction(ctx, opsJSON); return err },
			"db_query_batch": func() error {
				_, err := h.DBQueryBatch(ctx, opsJSON)
				return err
			},
			"exec_and_publish": func() error {
				_, err := h.ExecAndPublish(pubCtx, opsJSON, "wake", []byte("{}"))
				return err
			},
		}
		for fn, call := range calls {
			var refused *sqlguard.ErrNotAllowed
			if err := call(); !errors.As(err, &refused) {
				t.Errorf("%s(%s): err = %v, want the SQL guard's refusal", fn, sql, err)
			}
		}
	}
}

// A protected table hidden behind an innocent first op still has to be caught.
func TestDBTransaction_checksEveryOp(t *testing.T) {
	h := &HostFunctions{logger: zap.NewNop(), db: refusingDB{}, dbNamespace: testNamespace}
	_, err := h.DBTransaction(nsCtx(),
		[]byte(`{"ops":[{"kind":"exec","sql":"INSERT INTO messages(body) VALUES ('x')"},{"kind":"exec","sql":"UPDATE api_keys SET scopes='admin'"}]}`))
	if err == nil {
		t.Fatal("a protected table in the second op ran")
	}
	if !strings.Contains(err.Error(), "op 1") {
		t.Errorf("the refusal does not say which op: %v", err)
	}
}

// bugboard chg-943: a namespace's migrate function applies its schema through
// these host calls, so a trigger on a tenant table that touches only tenant
// tables reaches the database through db_execute_v2 and in a db_transaction
// batch, and one on or reaching a platform table is refused before it.
func TestDBHostFunctions_triggers(t *testing.T) {
	const tenant = "CREATE TRIGGER message_gone AFTER DELETE ON messages BEGIN DELETE FROM body_owed WHERE message_id = old.id; END"
	ctx := nsCtx()

	if _, err := newHFWithDB(&fakeExecClient{}).DBExecuteV2(ctx, tenant, nil); err != nil {
		t.Errorf("db_execute_v2 refused a tenant trigger: %v", err)
	}
	batch := &fakeBatchClient{}
	opsJSON, _ := json.Marshal(map[string]any{"ops": []map[string]any{{"kind": "exec", "sql": tenant}}})
	if _, err := newHFWithDB(batch).DBTransaction(ctx, opsJSON); err != nil || batch.calls != 1 {
		t.Errorf("db_transaction refused a tenant trigger: calls=%d err=%v", batch.calls, err)
	}

	h := &HostFunctions{logger: zap.NewNop(), db: refusingDB{}, dbNamespace: testNamespace}
	for _, sql := range []string{
		"CREATE TRIGGER t AFTER INSERT ON api_keys BEGIN DELETE FROM messages; END",
		"CREATE TRIGGER t AFTER INSERT ON messages BEGIN UPDATE grants SET role = 'owner'; END",
		"CREATE TRIGGER t AFTER INSERT ON messages BEGIN PRAGMA writable_schema = 1; END",
		"CREATE TEMP TRIGGER t AFTER INSERT ON messages BEGIN DELETE FROM body_owed; END",
	} {
		opsJSON, _ := json.Marshal(map[string]any{"ops": []map[string]any{{"kind": "exec", "sql": sql}}})
		var refused *sqlguard.ErrNotAllowed
		if _, err := h.DBExecuteV2(ctx, sql, nil); !errors.As(err, &refused) {
			t.Errorf("db_execute_v2(%s): err = %v, want the SQL guard's refusal", sql, err)
		}
		if _, err := h.DBTransaction(ctx, opsJSON); !errors.As(err, &refused) {
			t.Errorf("db_transaction(%s): err = %v, want the SQL guard's refusal", sql, err)
		}
	}
	_, err := h.DBExecuteV2(ctx, "CREATE TRIGGER t AFTER INSERT ON messages BEGIN DELETE FROM body_owed; DELETE FROM api_keys; END", nil)
	if err == nil || !strings.Contains(err.Error(), "trigger body statement 2 (DELETE FROM api_keys)") {
		t.Errorf("the refusal does not name the body statement: %v", err)
	}
}
