package tomb_test

import (
	"errors"
	"launchpad.net/tomb"
	"reflect"
	"testing"
)

func TestNewTomb(t *testing.T) {
	tb := new(tomb.Tomb)
	testState(t, tb, false, false, tomb.ErrStillRunning)

	tb.Done()
	testState(t, tb, true, true, nil)
}

func TestKill(t *testing.T) {
	// a nil reason flags the goroutine as dying
	tb := new(tomb.Tomb)
	tb.Kill(nil)
	testState(t, tb, true, false, nil)

	// a non-nil reason now will override Kill
	err := errors.New("some error")
	tb.Kill(err)
	testState(t, tb, true, false, err)

	// another non-nil reason won't replace the first one
	tb.Kill(errors.New("ignore me"))
	testState(t, tb, true, false, err)

	tb.Done()
	testState(t, tb, true, true, err)
}

func TestKillf(t *testing.T) {
	tb := new(tomb.Tomb)

	err := errors.New("BOOM")
	tb.Killf("BO%s", "OM")
	testState(t, tb, true, false, err)

	// another non-nil reason won't replace the first one
	tb.Killf("ignore me")
	testState(t, tb, true, false, err)

	tb.Done()
	testState(t, tb, true, true, err)
}

func testState(t *testing.T, tb *tomb.Tomb, wantDying, wantDead bool, wantErr error) {
	select {
	case <-tb.Dying():
		if !wantDying {
			t.Error("<-Dying: should block")
		}
	default:
		if wantDying {
			t.Error("<-Dying: should not block")
		}
	}
	seemsDead := false
	select {
	case <-tb.Dead():
		if !wantDead {
			t.Error("<-Dead: should block")
		}
		seemsDead = true
	default:
		if wantDead {
			t.Error("<-Dead: should not block")
		}
	}
	if err := tb.Err(); !reflect.DeepEqual(err, wantErr) {
		t.Errorf("Err: want %#v, got %#v", wantErr, err)
	}
	if wantDead && seemsDead {
		waitErr := tb.Wait()
		switch {
		case waitErr == tomb.ErrStillRunning:
			t.Errorf("Wait should not return ErrStillRunning")
		case !reflect.DeepEqual(waitErr, wantErr):
			t.Errorf("Wait: want %#v, got %#v", wantErr, waitErr)
		}
	}
}
