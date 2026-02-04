package main

import "github.com/strantalis/workset/pkg/worksetapi"

func newWorksetService() *worksetapi.Service {
	return worksetapi.NewService(serviceOptions())
}

func (a *App) ensureService() *worksetapi.Service {
	if a.service == nil {
		a.service = newWorksetService()
	}
	return a.service
}
