package tomb_test

import (
	"launchpad.net/tomb"
	"os"
	"testing"
)

func TestNewTomb(t *testing.T) {
	tb := tomb.New()
	testState(t, tb, false, false, nil)

	tb.Done()
	testState(t, tb, true, true, nil)
}

func TestFatal(t *testing.T) {
	tb := tomb.New()

	// a nil reason still puts it in dying mode
	tb.Fatal(nil)
	testState(t, tb, true, false, nil)

	// a non-nil reason now will override the nil reason
	err := os.NewError("some error")
	tb.Fatal(err)
	testState(t, tb, true, false, err)

	// another non-nil reason won't replace the first one
	tb.Fatal(os.NewError("ignore me"))
	testState(t, tb, true, false, err)

	tb.Done()
	testState(t, tb, true, true, err)
}

func TestFatalf(t *testing.T) {
	tb := tomb.New()

	err := os.NewError("BOOM")
	tb.Fatalf("BO%s", "OM")
	testState(t, tb, true, false, err)

	// another non-nil reason won't replace the first one
	tb.Fatalf("ignore me")
	testState(t, tb, true, false, err)

	tb.Done()
	testState(t, tb, true, true, err)
}

func testState(t *testing.T, tb *tomb.Tomb, wantDying, wantDead bool, wantErr os.Error) {
	if tb.IsDying() != wantDying {
		t.Errorf("IsDying: want %v, got %v", wantDying, !wantDying)
	}
	if tb.IsDead() != wantDead {
		t.Errorf("IsDead: want %v, got %v", wantDead, !wantDead)
	}
	select {
	case <-tb.Dying:
		if !wantDying {
			t.Error("<-Dying: should block")
		}
	default:
		if wantDying {
			t.Error("<-Dying: should not block")
		}
	}
	select {
	case <-tb.Dead:
		if !wantDead {
			t.Error("<-Dead: should block")
		}
	default:
		if wantDead {
			t.Error("<-Dead: should not block")
		}
	}
	if err := tb.Err(); err != wantErr {
		t.Errorf("Err: want %#v, got %#v", wantErr, err)
	}
}
