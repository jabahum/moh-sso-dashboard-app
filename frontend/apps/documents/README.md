# Documents App

The documents app handles document upload and document processing workflows.

It is a standalone React microfrontend exposed as `@moh-sso/documents` and mounted by the shell at two routes (`apps/shell/src/app/routes/user.routes.tsx`):

- `/apps/dwh/documents`: requires the `data-statistics` system, `systems:launch`, and
  `documents:read`.
- `/apps/utilities/self-service/eservice/document-upload`: requires the `utilities`
  system, `systems:launch`, and `documents:write`.

The app also accepts the legacy `/apps/dwh/filesvr` base when it is loaded there.

## Responsibilities

- Upload files and documents.
- Display document records and a document detail page (`/:id`).
- Support document processing flows.
- Manage document templates in the Templates tab.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:documents` starts the Vite dev server on port 4106;
`npm run preview:mf:documents` serves the built bundle on the same port.

```bash
npm run dev:mf:documents
npm run build -w @moh-sso/documents
npm run preview:mf:documents
```
