package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/seakee/cpa-manager/usage-service/internal/usage"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})
	return db
}

func TestSQLiteSetupRoundTrip(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	setup := Setup{
		CPAUpstreamURL: "http://localhost:8317",
		ManagementKey:  "secret",
		Queue:          "usage",
		PopSide:        "right",
	}
	if err := db.SaveSetup(ctx, setup); err != nil {
		t.Fatalf("save setup: %v", err)
	}
	loaded, ok, err := db.LoadSetup(ctx)
	if err != nil {
		t.Fatalf("load setup: %v", err)
	}
	if !ok {
		t.Fatal("expected setup to exist")
	}
	if loaded != setup {
		t.Fatalf("loaded setup mismatch: got %+v want %+v", loaded, setup)
	}
}

func TestSQLiteInsertEventsSkipsDuplicates(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()
	latency := int64(123)
	event := usage.Event{
		RequestID:    "req-1",
		EventHash:    "event-1",
		TimestampMS:  time.Date(2026, 5, 8, 1, 2, 3, 0, time.UTC).UnixMilli(),
		Timestamp:    "2026-05-08T01:02:03Z",
		Provider:     "claude",
		Model:        "claude-opus-4-6",
		Endpoint:     "POST /v1/messages",
		Method:       "POST",
		Path:         "/v1/messages",
		AuthType:     "claude",
		AuthIndex:    "0",
		Source:       "account@example.com",
		SourceHash:   "source-hash",
		APIKeyHash:   "key-hash",
		InputTokens:  10,
		OutputTokens: 20,
		TotalTokens:  30,
		LatencyMS:    &latency,
		RawJSON:      `{"ok":true}`,
		CreatedAtMS:  time.Now().UnixMilli(),
	}

	result, err := db.InsertEvents(ctx, []usage.Event{event})
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if result.Inserted != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected first insert result: %+v", result)
	}

	result, err = db.InsertEvents(ctx, []usage.Event{event})
	if err != nil {
		t.Fatalf("insert duplicate event: %v", err)
	}
	if result.Inserted != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected duplicate insert result: %+v", result)
	}

	events, err := db.RecentEvents(ctx, 10)
	if err != nil {
		t.Fatalf("recent events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventHash != event.EventHash || events[0].Model != event.Model || events[0].TotalTokens != event.TotalTokens {
		t.Fatalf("stored event mismatch: got %+v want %+v", events[0], event)
	}
}

func TestSQLiteModelPricesRoundTripAndSync(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	prices := map[string]ModelPrice{
		"claude-opus-4-6": {
			Prompt:        15,
			Completion:    75,
			Cache:         1.5,
			Source:        "manual",
			SourceModelID: "claude-opus-4-6",
			RawJSON:       `{"source":"manual"}`,
		},
	}
	if err := db.SaveModelPrices(ctx, prices); err != nil {
		t.Fatalf("save model prices: %v", err)
	}
	loaded, err := db.LoadModelPrices(ctx)
	if err != nil {
		t.Fatalf("load model prices: %v", err)
	}
	if loaded["claude-opus-4-6"].Prompt != 15 || loaded["claude-opus-4-6"].Source != "manual" {
		t.Fatalf("loaded model price mismatch: %+v", loaded["claude-opus-4-6"])
	}

	result, err := db.UpsertSyncedModelPrices(ctx, map[string]ModelPrice{
		"claude-opus-4-6": {Prompt: 16, Completion: 80, Cache: 2},
		"":                {Prompt: 1, Completion: 1, Cache: 1},
	})
	if err != nil {
		t.Fatalf("sync model prices: %v", err)
	}
	if result.Imported != 1 || result.Skipped != 1 {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	loaded, err = db.LoadModelPrices(ctx)
	if err != nil {
		t.Fatalf("load synced model prices: %v", err)
	}
	price := loaded["claude-opus-4-6"]
	if price.Prompt != 16 || price.Source != "sync" || price.SyncedAtMS == nil {
		t.Fatalf("synced model price mismatch: %+v", price)
	}
}

func TestSQLiteDeadLetterCounts(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	if err := db.AddDeadLetter(ctx, `{"bad":true}`, errors.New("parse failed")); err != nil {
		t.Fatalf("add dead letter: %v", err)
	}
	events, deadLetters, err := db.Counts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if events != 0 || deadLetters != 1 {
		t.Fatalf("unexpected counts: events=%d deadLetters=%d", events, deadLetters)
	}
}
