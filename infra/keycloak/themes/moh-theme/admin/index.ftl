<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <base href="${resourceUrl}/">
    <link rel="icon" type="${properties.favIconType!'image/png'}" href="${resourceUrl}${properties.favIcon!'/img/logo.png'}">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="description" content="${properties.description!'Administer Ministry of Health identity and access management.'}">
    <title>${properties.title!'MOH Identity Administration'}</title>
    <style>
      body {
        margin: 0;
        background: #f4f4f4;
        color: #161616;
        font-family: "IBM Plex Sans", "Segoe UI", Arial, sans-serif;
      }

      body,
      #app {
        min-height: 100%;
      }

      .container {
        padding: 0;
        margin: 0;
        width: 100%;
      }

      .keycloak__loading-container {
        height: 100vh;
        width: 100%;
        color: #161616;
        background: #f4f4f4;
        display: flex;
        align-items: center;
        justify-content: center;
        flex-direction: column;
        gap: 1rem;
        margin: 0;
      }

      .moh-console-loading-logo {
        width: 64px;
        height: 64px;
        object-fit: contain;
      }

      #loading-text {
        z-index: 1000;
        font-size: 20px;
        font-weight: 600;
        margin: 0;
      }

      .moh-console-loading-subtext {
        margin: 0;
        color: #525252;
      }
    </style>
    <script type="importmap">
      {
        "imports": {
          "react": "${resourceCommonUrl}/vendor/react/react.production.min.js",
          "react/jsx-runtime": "${resourceCommonUrl}/vendor/react/react-jsx-runtime.production.min.js",
          "react-dom": "${resourceCommonUrl}/vendor/react-dom/react-dom.production.min.js"
        }
      }
    </script>
    <#if !isSecureContext>
      <script type="module" src="${resourceCommonUrl}/vendor/web-crypto-shim/web-crypto-shim.js"></script>
    </#if>
    <#if devServerUrl?has_content>
      <script type="module">
        import { injectIntoGlobalHook } from "${devServerUrl}/@react-refresh";

        injectIntoGlobalHook(window);
        window.$RefreshReg$ = () => {};
        window.$RefreshSig$ = () => (type) => type;
      </script>
      <script type="module">
        import { inject } from "${devServerUrl}/@vite-plugin-checker-runtime";

        inject({
          overlayConfig: {},
          base: "/",
        });
      </script>
      <script type="module" src="${devServerUrl}/@vite/client"></script>
      <script type="module" src="${devServerUrl}/src/main.tsx"></script>
    </#if>
    <#if entryStyles?has_content>
      <#list entryStyles as style>
        <link rel="stylesheet" href="${resourceUrl}/${style}">
      </#list>
    </#if>
    <#if properties.styles?has_content>
      <#list properties.styles?split(' ') as style>
        <link rel="stylesheet" href="${resourceUrl}/${style}">
      </#list>
    </#if>
    <#if entryScript?has_content>
      <script type="module" src="${resourceUrl}/${entryScript}"></script>
    </#if>
    <#if properties.scripts?has_content>
      <#list properties.scripts?split(' ') as script>
        <script type="module" src="${resourceUrl}/${script}"></script>
      </#list>
    </#if>
    <#if entryImports?has_content>
      <#list entryImports as import>
        <link rel="modulepreload" href="${resourceUrl}/${import}">
      </#list>
    </#if>
  </head>
  <body class="moh-admin-console">
    <div id="app">
      <main class="container">
        <div class="keycloak__loading-container">
          <img class="moh-console-loading-logo" src="${resourceUrl}/img/logo.png" alt="MOH Logo">
          <p id="loading-text">Loading MOH Identity Administration</p>
          <p class="moh-console-loading-subtext">Preparing realm, users, clients, and access controls.</p>
        </div>
      </main>
    </div>
    <noscript>JavaScript is required to use the MOH Identity Administration Console.</noscript>
    <script id="environment" type="application/json">
      {
        "serverBaseUrl": "${serverBaseUrl}",
        "adminBaseUrl": "${adminBaseUrl}",
        "authUrl": "${authUrl}",
        "authServerUrl": "${authServerUrl}",
        "realm": "${loginRealm!"master"}",
        "clientId": "${clientId}",
        "resourceUrl": "${resourceUrl}",
        "logo": "${properties.logo!""}",
        "logoUrl": "${properties.logoUrl!""}",
        "consoleBaseUrl": "${consoleBaseUrl}",
        "masterRealm": "${masterRealm}",
        "resourceVersion": "${resourceVersion}"
      }
    </script>
  </body>
</html>
