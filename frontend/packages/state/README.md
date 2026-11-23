# State Package

`@moh-sso/state` owns the shared Redux store and global frontend state slices.

## Responsibilities

- Configure the global Redux store.
- Register RTK Query middleware.
- Hold the `auth` and `clients` slices (registered clients and the active client) next to the
  `baseApi` reducer.
- Export selectors shared by shell and apps.

## Usage

```ts
import { store, selectClients } from "@moh-sso/state";
```

## Development

```bash
npm run build -w @moh-sso/state
```
