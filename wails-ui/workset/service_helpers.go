package main

import "github.com/strantalis/workset/pkg/worksetapi"

func newWorksetService(observer appHookObserver) *worksetapi.Service {
	options := serviceOptions()
	options.HookObserver = observer
	return worksetapi.NewService(options)
}

func (a *App) ensureService() *worksetapi.Service {
	a.serviceOnce.Do(func() {
		if a.service == nil {
			a.service = newWorksetService(appHookObserver{app: a})
		}
	})
	return a.service
}
