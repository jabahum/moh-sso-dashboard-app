---
"@moh-sso/api": patch
"@moh-sso/auth": patch
"@moh-sso/config": patch
"@moh-sso/microfrontend": patch
"@moh-sso/state": patch
"@moh-sso/types": patch
"@moh-sso/ui": patch
"@moh-sso/utils": patch
---

Keep React and React DOM subpaths, including the client renderer and JSX runtimes,
external in published shared packages. Microfrontends now use the import map's
matching React runtime instead of loading a renderer bundled at build time, which
could fail every published application with React version mismatch error 527.
