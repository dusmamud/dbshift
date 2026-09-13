package core_test

import (
	"errors"
	"testing"

	"github.com/dusmamud/dbshift/internal/core"
)

func TestDefaultOptions(t *testing.T) {
	opts := core.DefaultOptions()
	if opts.BatchSize != 2500 {
		t.Errorf("expected batch size 2500, got %d", opts.BatchSize)
	}
	if opts.Concurrency != 4 {
		t.Errorf("expected concurrency 4, got %d", opts.Concurrency)
	}
	if opts.Mode != core.ModeFull {
		t.Errorf("expected mode full, got %s", opts.Mode)
	}
}

func TestDbmError(t *testing.T) {
	baseErr := errors.New("network timeout")
	err := core.NewError(core.EnginePostgres, "StreamCopy", "connection dropped", baseErr)

	if err.Error() != "[postgres/StreamCopy] connection dropped: network timeout" {
		t.Errorf("unexpected error string: %s", err.Error())
	}

	var dbmErr *core.DbmError
	if !errors.As(err, &dbmErr) {
		t.Fatalf("expected error to be *core.DbmError")
	}

	if dbmErr.Engine != core.EnginePostgres {
		t.Errorf("expected engine postgres, got %s", dbmErr.Engine)
	}
}
