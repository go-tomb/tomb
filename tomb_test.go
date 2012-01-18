package tomb_test

import (
	"errors"
	"launchpad.net/tomb"
	"reflect"

	"testing"
)

func TestNewTomb(t *testing.T) {
	tb := tomb.New()
	testState(t, tb, false, false, nil)

	tb.Done()
	testState(t, tb, true, true, tomb.Stop)
}

func TestFatal(t *testing.T) {
	tb := tomb.New()

	// the Stop reason flags the goroutine as dying
	tb = tomb.New()
	tb.Fatal(tomb.Stop)
	testState(t, tb, true, false, tomb.Stop)

	// a non-Stop reason now will override Stop
	err := errors.New("some error")
	tb.Fatal(err)
	testState(t, tb, true, false, err)

	// another non-nil reason won't replace the first one
	tb.Fatal(errors.New("ignore me"))
	testState(t, tb, true, false, err)

	tb.Done()
	testState(t, tb, true, true, err)
}

func TestFatalf(t *testing.T) {
	tb := tomb.New()

	err := errors.New("BOOM")
	tb.Fatalf("BO%s", "OM")
	testState(t, tb, true, false, err)

	// another non-Stop reason won't replace the first one
	tb.Fatalf("ignore me")
	testState(t, tb, true, false, err)

	tb.Done()
	testState(t, tb, true, true, err)
}

func testState(t *testing.T, tb *tomb.Tomb, wantDying, wantDead bool, wantErr error) {
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
	seemsDead := false
	select {
	case <-tb.Dead:
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
		if wantErr == tomb.Stop {
			if waitErr != nil {
				t.Errorf("Wait: want nil, got %#v", waitErr)
			}
		} else if !reflect.DeepEqual(waitErr, wantErr) {
			t.Errorf("Wait: want %#v, got %#v", wantErr, waitErr)
		}
	}
}
