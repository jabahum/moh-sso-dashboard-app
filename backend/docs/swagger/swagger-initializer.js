/* global window, SwaggerUIBundle */
window.onload = function () {
  window.ui = SwaggerUIBundle({
    url: "./openapi.json",
    dom_id: "#swagger-ui",
    deepLinking: true,
    filter: true,
    docExpansion: "none",
    displayRequestDuration: true,
    withCredentials: true,
    persistAuthorization: false,
    validatorUrl: null,
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout",
  });
};
