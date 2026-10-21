# Local Express integration stack

Start Docker Desktop, then run from the repository root:

```bash
docker compose -f docker-compose.dev.yml -f .local-sso/compose.express.json up -d --no-build backend
```

Start the frontend in another terminal using Node 22+:

```bash
cd frontend
npm run portal -- serve --backend http://127.0.0.1:9000
```

Open http://localhost:3000/portal/. The backend uses the separate `sso_express_dev`
database because the previous database has migrations newer than this checkout.
Initial setup seeds reference data and can take several minutes. Existing database
volumes are preserved. The local Keycloak dashboard client allows the Express callback
at http://localhost:3000/backend/api/v1/auth/callback.

The backend image is the cached development image; Air rebuilds the bind-mounted source.
This local configuration and realm export are excluded from Git.
