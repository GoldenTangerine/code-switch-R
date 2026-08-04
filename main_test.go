package main

import (
	_ "modernc.org/sqlite"
	"testing"
	"time"
)

type mainWindowTestTimer struct {
	stopped  bool
	callback func()
}

func (timer *mainWindowTestTimer) Stop() bool {
	wasActive := !timer.stopped
	timer.stopped = true
	return wasActive
}

type mainWindowTestScheduler struct {
	timers []*mainWindowTestTimer
}

func (scheduler *mainWindowTestScheduler) afterFunc(_ time.Duration, callback func()) mainWindowDestroyTimer {
	timer := &mainWindowTestTimer{callback: callback}
	scheduler.timers = append(scheduler.timers, timer)
	return timer
}

func (scheduler *mainWindowTestScheduler) fire(index int) {
	if index < 0 || index >= len(scheduler.timers) {
		return
	}
	scheduler.timers[index].callback()
}

func TestMainWindowLifecycleStateCancelsStaleDestroy(t *testing.T) {
	scheduler := &mainWindowTestScheduler{}
	state := newMainWindowLifecycleState[string](scheduler.afterFunc)
	state.setWindow("first")

	closed := make([]string, 0, 1)
	state.scheduleDestroy("first", 1, time.Second, func(window string) {
		closed = append(closed, window)
	})
	state.scheduleDestroy("first", 1, time.Second, func(window string) {
		closed = append(closed, window)
	})
	scheduler.fire(0)
	if len(closed) != 0 {
		t.Fatalf("旧定时器不应关闭窗口: %v", closed)
	}
	scheduler.fire(1)
	if len(closed) != 1 || closed[0] != "first" {
		t.Fatalf("当前定时器关闭结果 = %v, want [first]", closed)
	}
	if _, exists := state.current(); exists {
		t.Fatal("销毁后仍保留当前窗口")
	}
	if !state.allowClose(1) || state.allowClose(1) {
		t.Fatal("关闭许可应当只消费一次")
	}
}

func TestMainWindowLifecycleStateShutdownCancelsDestroy(t *testing.T) {
	scheduler := &mainWindowTestScheduler{}
	state := newMainWindowLifecycleState[string](scheduler.afterFunc)
	state.setWindow("first")
	closed := false
	state.scheduleDestroy("first", 1, time.Second, func(string) {
		closed = true
	})
	state.shutdown()
	scheduler.fire(0)
	if closed {
		t.Fatal("应用退出后不应执行销毁回调")
	}
	if !state.allowClose(1) {
		t.Fatal("应用退出后应允许原生关闭事件通过")
	}
}

func TestMainWindowLifecycleStateReopenCancelsDestroy(t *testing.T) {
	scheduler := &mainWindowTestScheduler{}
	state := newMainWindowLifecycleState[string](scheduler.afterFunc)
	state.setWindow("first")
	closed := false
	state.scheduleDestroy("first", 1, time.Second, func(string) {
		closed = true
	})
	state.cancelDestroy()
	scheduler.fire(0)
	window, exists := state.current()
	if closed || !exists || window != "first" {
		t.Fatalf("重开取消销毁后的状态异常: closed=%v, exists=%v, window=%q", closed, exists, window)
	}
}

func TestResolveTrayWindowHeightUsesContentAndScreenBounds(t *testing.T) {
	tests := []struct {
		name           string
		contentHeight  int
		workAreaHeight int
		wantHeight     int
		wantMaxHeight  int
	}{
		{name: "content", contentHeight: 760, workAreaHeight: 900, wantHeight: 760, wantMaxHeight: 876},
		{name: "screen limit", contentHeight: 920, workAreaHeight: 900, wantHeight: 876, wantMaxHeight: 876},
		{name: "minimum", contentHeight: 80, workAreaHeight: 900, wantHeight: 120, wantMaxHeight: 876},
		{name: "small screen", contentHeight: 760, workAreaHeight: 100, wantHeight: 120, wantMaxHeight: 120},
		{name: "fallback", contentHeight: 760, workAreaHeight: 0, wantHeight: 760, wantMaxHeight: 1200},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			height, maxHeight := resolveTrayWindowHeight(test.contentHeight, test.workAreaHeight)
			if height != test.wantHeight || maxHeight != test.wantMaxHeight {
				t.Fatalf("resolveTrayWindowHeight(%d, %d) = (%d, %d), want (%d, %d)",
					test.contentHeight, test.workAreaHeight, height, maxHeight, test.wantHeight, test.wantMaxHeight)
			}
		})
	}
}
