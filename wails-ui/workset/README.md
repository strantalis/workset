# Workset Desktop App

## About

This is the Workset desktop UI built with Wails (Go backend + Svelte frontend).

## Live Development

To run in live development mode, run `wails3 dev` in the project directory. This starts a Vite dev server
with hot reload for the frontend, plus a Wails backend bridge. If you want to develop in a browser and
have access to your Go methods, connect to the URL printed by the dev runner.

Dev mode isolates config, workspaces, repo store, and UI state under `~/.workset/dev`.

## Building

To build a redistributable, production mode package, use `wails3 package`.
