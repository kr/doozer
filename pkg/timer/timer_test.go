package timer

import (
	"junta/assert"
	"junta/store"
	"testing"
	"runtime"
	"time"
	"strconv"
)

const (
	testPattern = "/timer/**"
)

func encodeTimer(path string, offset int64) string {
	future := time.Nanoseconds() + offset
	muta := store.MustEncodeSet(
		path,
		strconv.Itoa64(future),
		store.Clobber,
	)
	return muta
}

func TestManyOneshotTimers(t *testing.T) {
	st := store.New()
	timer := New(testPattern, 10*OneMillisecond, st)
	defer timer.Close()

	st.Apply(1, encodeTimer("/timer/longest", 40*OneMillisecond))
	st.Apply(2, encodeTimer("/timer/short", 10*OneMillisecond))
	st.Apply(3, encodeTimer("/timer/long", 25*OneMillisecond))

	got := <-timer.C
	assert.Equal(t, got.Path, "/timer/short")
	assert.T(t, got.At <= time.Nanoseconds())

	got = <-timer.C
	assert.Equal(t, got.Path, "/timer/long")
	assert.T(t, got.At <= time.Nanoseconds())

	got = <-timer.C
	assert.Equal(t, got.Path, "/timer/longest")
	assert.T(t, got.At <= time.Nanoseconds())

	assert.Equal(t, 0, timer.Len())
}

func TestDeleteTimer(t *testing.T) {
	st := store.New()
	timer := New(testPattern, 10*OneMillisecond, st)
	defer timer.Close()

	never := "/timer/never/ticks"

	watch := make(chan store.Event)
	st.Watch(testPattern, watch)

	// Wait one minute to ensure it doesn't tick before
	// the following delete and assert.
	st.Apply(1, encodeTimer(never, 60*OneSecond))
	<-watch

	st.Apply(2, store.MustEncodeDel(never, store.Clobber))
	<-watch

	// Make sure the timer goroutine has a chance to delete the timer.
	runtime.Gosched()

	assert.Equal(t, 0, timer.Len())
}

func TestUpdate(t *testing.T) {
	st := store.New()
	timer := New(testPattern, OneMillisecond, st)
	defer timer.Close()

	st.Apply(1, encodeTimer("/timer/y", 30*OneMillisecond))
	st.Apply(2, encodeTimer("/timer/x", 10*OneMillisecond))
	st.Apply(3, encodeTimer("/timer/x", 20*OneMillisecond))

	// The deadline scheduled from seqn 2 should never fire. It should be
	// replaced by seqn 3.

	assert.Equal(t, "/timer/x", (<-timer.C).Path) // From seqn 3
	assert.Equal(t, "/timer/y", (<-timer.C).Path) // From seqn 1
}
