# Coterie web client

React + TypeScript dashboard (design §7.5): Vite, Tailwind CSS,
TanStack Query, TanStack Router. Read-only views first — login,
circle list, and the circle detail (members / seats / billing
periods) — with write actions landing next.

## Develop

```bash
npm install
npm run dev        # http://localhost:5173
```

The dev server proxies `/api` to `http://localhost:8080` (see
`vite.config.ts`). Run the API alongside it — `go run ./apps/server`
with `DATABASE_URL` set, or the compose stack from the repo root.

## Build

```bash
npm run build      # typecheck + bundle into dist/
npm run lint       # oxlint
```

`dist/` is embedded into the server binary by the Docker build, so
the deployed stack serves the UI and the API from one port. The
generated `routeTree.gen.ts` is committed so CI can typecheck
without running the dev server first.
