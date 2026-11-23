# Multi-framework apps

Future frontend apps may use React, Vue, Angular, Svelte, or plain JavaScript. This page lists the lifecycle contract, adapters and rules every app must follow, whatever its framework. See also [non-React microfrontends](non-react-microfrontends.md).

Every app must expose the same lifecycle contract:

```ts
export async function bootstrap(props) {}
export async function mount(props) {}
export async function unmount(props) {}
```

Each app should accept `MicrofrontendRuntimeProps` from `@moh-sso/microfrontend`.

## Recommended adapters

- React: `single-spa-react`
- Vue: `single-spa-vue`
- Angular: `single-spa-angular`
- Svelte: `single-spa-svelte`
- Plain JavaScript: direct lifecycle **exports**

## Rules

- Apps must not import shell internals.
- Apps should communicate through shared **packages**, route props, or events.
- Cross-app dependencies should be intentional and documented.
- Shared UI, API, auth, config, state, and types should live in `frontend/packages`.
