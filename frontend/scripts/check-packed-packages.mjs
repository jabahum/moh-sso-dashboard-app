/* global console, process, URL */
import { execFileSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const root = fileURLToPath(new URL("../", import.meta.url));
const npm = process.env.NPM || "npm";
const modules = ["apps", "packages"].flatMap((group) =>
  readdirSync(join(root, group)).flatMap((name) => {
    const path = join(root, group, name);
    if (!existsSync(join(path, "package.json"))) return [];
    const pkg = JSON.parse(readFileSync(join(path, "package.json"), "utf8"));
    return pkg.private ? [] : [{ path, pkg }];
  }),
);
const fixture = mkdtempSync(join(tmpdir(), "moh-packed-consumer-"));
try {
  const installed = join(fixture, "node_modules");
  mkdirSync(installed);
  writeFileSync(join(fixture, "package.json"), JSON.stringify({ private: true, type: "module" }));
  // Supply third-party dependencies without network access. Internal packages
  // below are extracted exclusively from npm tarballs, never workspace links.
  for (const name of readdirSync(join(root, "node_modules"))) {
    if (name === "@moh-sso" || name.startsWith(".")) continue;
    symlinkSync(join(root, "node_modules", name), join(installed, name));
  }
  const imports = [];
  for (const { path, pkg } of modules) {
    const packed = JSON.parse(
      execFileSync(
        npm,
        [
          "pack",
          path,
          "--ignore-scripts",
          "--offline",
          "--json",
          "--pack-destination",
          fixture,
          "--cache",
          join(fixture, "cache"),
        ],
        { cwd: root, encoding: "utf8" },
      ),
    )[0];
    const destination = join(installed, pkg.name);
    mkdirSync(destination, { recursive: true });
    execFileSync("tar", [
      "-xzf",
      join(fixture, packed.filename),
      "--strip-components=1",
      "-C",
      destination,
    ]);
    for (const [subpath, entry] of Object.entries(pkg.exports)) {
      const targets = typeof entry === "string" ? [entry] : Object.values(entry);
      for (const target of targets) {
        if (!existsSync(join(destination, target)))
          throw new Error(`${pkg.name}: packed export ${target} is missing`);
      }
      if (typeof entry === "string") continue; // CSS exports
      const specifier = pkg.name + (subpath === "." ? "" : subpath.slice(1));
      imports.push(
        `import * as module${imports.length} from ${JSON.stringify(specifier)};\nvoid module${imports.length};`,
      );
    }
  }
  const consumer = join(fixture, "consumer.ts");
  writeFileSync(
    consumer,
    imports.join("\n") +
      `
import type { ClientRole } from "@moh-sso/types";
const role: ClientRole = { id: "fixture", name: "Reader" };
// @ts-expect-error Published field types must reject numbers, rather than any.
role.name = 42;
void role;
`,
  );
  const options = {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    strict: true,
    skipLibCheck: true,
    noEmit: true,
    jsx: ts.JsxEmit.ReactJSX,
    types: [],
    allowSyntheticDefaultImports: true,
  };
  const program = ts.createProgram([consumer], options);
  const diagnostics = ts.getPreEmitDiagnostics(program);
  if (diagnostics.length)
    throw new Error(
      ts.formatDiagnosticsWithColorAndContext(diagnostics, {
        getCanonicalFileName: (file) => file,
        getCurrentDirectory: () => fixture,
        getNewLine: () => "\n",
      }),
    );
  // skipLibCheck avoids unrelated third-party declaration errors. Independently
  // verify every import in our packed declaration graph, including private files.
  const walk = (directory) =>
    readdirSync(directory, { withFileTypes: true }).flatMap((entry) =>
      entry.isDirectory() ? walk(join(directory, entry.name)) : [join(directory, entry.name)],
    );
  for (const { pkg } of modules) {
    for (const file of walk(join(installed, pkg.name)).filter((path) => path.endsWith(".d.ts"))) {
      const source = ts.createSourceFile(
        file,
        readFileSync(file, "utf8"),
        ts.ScriptTarget.Latest,
        true,
      );
      const visit = (node) => {
        const literal =
          ts.isImportDeclaration(node) || ts.isExportDeclaration(node)
            ? node.moduleSpecifier
            : ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument)
              ? node.argument.literal
              : undefined;
        if (literal && ts.isStringLiteral(literal)) {
          const specifier = literal.text;
          if (
            !/\.(css|scss)$/.test(specifier) &&
            !ts.resolveModuleName(specifier, file, options, ts.sys).resolvedModule
          ) {
            throw new Error(
              `${pkg.name}: unresolved packed declaration import ${specifier} in ${file}`,
            );
          }
        }
        ts.forEachChild(node, visit);
      };
      visit(source);
    }
  }
  console.log(
    `Verified ${modules.length} npm tarballs and ${imports.length} typed consumer imports without workspace aliases or registry writes.`,
  );
} finally {
  rmSync(fixture, { recursive: true, force: true });
}
