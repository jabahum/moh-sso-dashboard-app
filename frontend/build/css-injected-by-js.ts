import type { Plugin } from "vite";

type CssInJsOptions = {
  topExecutionPriority?: boolean;
};

function buildInjectionRuntime(cssFiles: string[], topExecutionPriority: boolean): string {
  const entries = cssFiles
    .map((file) => `new URL(${JSON.stringify(file)}, import.meta.url).href`)
    .join(", ");

  return `
const __mohCssFiles = [${entries}];
const __mohInjectCss = () => {
  if (typeof document === "undefined") {
    return;
  }

  for (const file of __mohCssFiles) {
    const href = file;
    if (document.querySelector('link[data-moh-css="' + href + '"]')) {
      continue;
    }

    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = href;
    link.setAttribute("data-moh-css", href);
    document.head.appendChild(link);
  }
};
${topExecutionPriority ? "__mohInjectCss();\n" : "queueMicrotask(__mohInjectCss);\n"}
`;
}

export default function cssInjectedByJsPlugin(options: CssInJsOptions = {}): Plugin {
  const topExecutionPriority = options.topExecutionPriority ?? false;

  return {
    name: "moh-css-injected-by-js",
    enforce: "post",
    generateBundle: {
      order: "post",
      handler(_outputOptions, bundle) {
        const cssFiles = Object.values(bundle)
          .filter((entry) => entry.type === "asset" && entry.fileName.endsWith(".css"))
          .map((entry) => entry.fileName);

        if (cssFiles.length === 0) {
          return;
        }

        for (const entry of Object.values(bundle)) {
          if (entry.type !== "chunk" || !entry.isEntry) {
            continue;
          }

          entry.code = `${buildInjectionRuntime(cssFiles, topExecutionPriority)}\n${entry.code}`;
        }
      },
    },
  };
}
