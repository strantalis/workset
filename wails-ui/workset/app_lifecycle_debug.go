package main

import (
	"fmt"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func attachLifecycleDebugLogging(app *application.App, mainWindow *application.WebviewWindow) {
	if app == nil || mainWindow == nil {
		return
	}

	logAppEvent := func(eventType events.ApplicationEventType, event *application.ApplicationEvent) {
		details := fmt.Sprintf(
			"name=%s pid=%d visible_windows=%d total_windows=%d",
			lifecycleApplicationEventName(eventType),
			os.Getpid(),
			countVisibleWindows(app.Window.GetAll()),
			len(app.Window.GetAll()),
		)
		if event != nil && eventType == events.Mac.ApplicationShouldHandleReopen {
			details += fmt.Sprintf(" has_visible_windows=%t", event.Context().HasVisibleWindows())
		}
		debugTerminalServicef("app_event %s", details)
	}

	for _, eventType := range []events.ApplicationEventType{
		events.Common.ApplicationStarted,
		events.Mac.ApplicationDidBecomeActive,
		events.Mac.ApplicationDidHide,
		events.Mac.ApplicationDidResignActive,
		events.Mac.ApplicationDidUnhide,
		events.Mac.ApplicationShouldHandleReopen,
		events.Mac.ApplicationWillHide,
		events.Mac.ApplicationWillTerminate,
		events.Mac.ApplicationWillUnhide,
	} {
		eventType := eventType
		app.Event.OnApplicationEvent(eventType, func(event *application.ApplicationEvent) {
			logAppEvent(eventType, event)
		})
	}

	logWindowEvent := func(eventType events.WindowEventType, event *application.WindowEvent) {
		debugTerminalServicef(
			"window_event name=%s window=%s pid=%d main_visible=%t visible_windows=%d total_windows=%d cancelled=%t",
			lifecycleWindowEventName(eventType),
			mainWindowName,
			os.Getpid(),
			mainWindow.IsVisible(),
			countVisibleWindows(app.Window.GetAll()),
			len(app.Window.GetAll()),
			event.IsCancelled(),
		)
	}

	for _, eventType := range []events.WindowEventType{
		events.Common.WindowClosing,
		events.Common.WindowFocus,
		events.Common.WindowHide,
		events.Common.WindowLostFocus,
		events.Common.WindowRuntimeReady,
		events.Common.WindowShow,
		events.Mac.WindowWillClose,
		events.Mac.WindowDidResignKey,
		events.Mac.WindowDidBecomeKey,
		events.Mac.WindowDidMiniaturize,
		events.Mac.WindowDidDeminiaturize,
		events.Mac.WindowDidOrderOffScreen,
		events.Mac.WindowDidOrderOnScreen,
	} {
		eventType := eventType
		mainWindow.RegisterHook(eventType, func(event *application.WindowEvent) {
			logWindowEvent(eventType, event)
		})
	}
}

func preventMainWindowHideOnFocusLost(mainWindow *application.WebviewWindow) {
	if mainWindow == nil {
		return
	}
	mainWindow.RegisterHook(events.Common.WindowLostFocus, func(event *application.WindowEvent) {
		debugTerminalServicef(
			"window_event_prevented name=%s window=%s pid=%d reason=prevent_focus_lost_hide",
			lifecycleWindowEventName(events.Common.WindowLostFocus),
			mainWindowName,
			os.Getpid(),
		)
		event.Cancel()
	})
}

func countVisibleWindows(windows []application.Window) int {
	count := 0
	for _, window := range windows {
		if window != nil && window.IsVisible() {
			count++
		}
	}
	return count
}

func lifecycleApplicationEventName(eventType events.ApplicationEventType) string {
	switch eventType {
	case events.Common.ApplicationStarted:
		return "common:ApplicationStarted"
	case events.Mac.ApplicationDidBecomeActive:
		return "mac:ApplicationDidBecomeActive"
	case events.Mac.ApplicationDidHide:
		return "mac:ApplicationDidHide"
	case events.Mac.ApplicationDidResignActive:
		return "mac:ApplicationDidResignActive"
	case events.Mac.ApplicationDidUnhide:
		return "mac:ApplicationDidUnhide"
	case events.Mac.ApplicationShouldHandleReopen:
		return "mac:ApplicationShouldHandleReopen"
	case events.Mac.ApplicationWillHide:
		return "mac:ApplicationWillHide"
	case events.Mac.ApplicationWillTerminate:
		return "mac:ApplicationWillTerminate"
	case events.Mac.ApplicationWillUnhide:
		return "mac:ApplicationWillUnhide"
	default:
		return fmt.Sprintf("application:%d", eventType)
	}
}

func lifecycleWindowEventName(eventType events.WindowEventType) string {
	switch eventType {
	case events.Common.WindowClosing:
		return "common:WindowClosing"
	case events.Common.WindowFocus:
		return "common:WindowFocus"
	case events.Common.WindowHide:
		return "common:WindowHide"
	case events.Common.WindowLostFocus:
		return "common:WindowLostFocus"
	case events.Common.WindowRuntimeReady:
		return "common:WindowRuntimeReady"
	case events.Common.WindowShow:
		return "common:WindowShow"
	case events.Mac.WindowWillClose:
		return "mac:WindowWillClose"
	case events.Mac.WindowDidResignKey:
		return "mac:WindowDidResignKey"
	case events.Mac.WindowDidBecomeKey:
		return "mac:WindowDidBecomeKey"
	case events.Mac.WindowDidMiniaturize:
		return "mac:WindowDidMiniaturize"
	case events.Mac.WindowDidDeminiaturize:
		return "mac:WindowDidDeminiaturize"
	case events.Mac.WindowDidOrderOffScreen:
		return "mac:WindowDidOrderOffScreen"
	case events.Mac.WindowDidOrderOnScreen:
		return "mac:WindowDidOrderOnScreen"
	default:
		return fmt.Sprintf("window:%d", eventType)
	}
}
