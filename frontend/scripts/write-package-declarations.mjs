/* global process, URL */
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const [, , kind, name] = process.argv;
if (!["app", "package"].includes(kind) || !name) {
  throw new Error("Usage: node scripts/write-package-declarations.mjs <app|package> <name>");
}
const root = fileURLToPath(new URL("../", import.meta.url));
const base = join(root, kind === "app" ? "apps" : "packages", name);
const pkg = JSON.parse(readFileSync(join(base, "package.json"), "utf8"));
const dist = join(base, "dist");
const out = join(dist, "_types");
const entries = Object.values(pkg.exports)
  .filter((entry) => entry?.types)
  .map((entry) => {
    const entryName = entry.types.replace("./dist/", "").replace(/\.d\.ts$/, "");
    const source = [
      join(base, "src", `${entryName}.ts`),
      join(base, "src", `${entryName}.tsx`),
      join(base, "src", entryName, "index.ts"),
    ].find(existsSync);
    if (!source) throw new Error(`Missing declaration source for ${pkg.name}/${entryName}`);
    return { entryName, source };
  });
const config = ts.readConfigFile(join(root, "tsconfig.base.json"), ts.sys.readFile);
if (config.error) throw new Error(ts.flattenDiagnosticMessageText(config.error.messageText, "\n"));
const { options } = ts.parseJsonConfigFileContent(config.config, ts.sys, root);
Object.assign(options, {
  noEmit: false,
  declaration: true,
  emitDeclarationOnly: true,
  rootDir: root,
  outDir: out,
  incremental: false,
  composite: false,
  noUnusedLocals: false,
  noUnusedParameters: false,
  types: [],
});
const program = ts.createProgram(
  [...entries.map((entry) => entry.source), join(root, "node_modules/vite/client.d.ts")],
  options,
);
const checker = program.getTypeChecker();
const diagnostics = ts.getPreEmitDiagnostics(program);
if (diagnostics.length) {
  throw new Error(
    ts.formatDiagnosticsWithColorAndContext(diagnostics, {
      getCanonicalFileName: (file) => file,
      getCurrentDirectory: () => root,
      getNewLine: () => "\n",
    }),
  );
}
// Ship the local declaration graph and remove workspace-only source aliases.
const rewriteAliases = (context) => (sourceFile) => {
  const rewrite = (literal) => {
    if (
      !literal ||
      !ts.isStringLiteral(literal) ||
      !(literal.text.startsWith("@moh-sso/") || literal.text.startsWith("@/"))
    )
      return literal;
    const target = ts.resolveModuleName(
      literal.text,
      sourceFile.fileName,
      options,
      ts.sys,
    ).resolvedModule;
    if (!target || target.isExternalLibraryImport)
      throw new Error(`Cannot bundle declaration import ${literal.text} in ${sourceFile.fileName}`);
    let path = relative(dirname(sourceFile.fileName), target.resolvedFileName)
      .replaceAll("\\", "/")
      .replace(/\.(tsx?|mts)$/, ".js");
    if (!path.startsWith(".")) path = `./${path}`;
    return context.factory.createStringLiteral(path);
  };
  const visit = (node) => {
    node = ts.visitEachChild(node, visit, context);
    if (ts.isImportDeclaration(node)) {
      if (!node.importClause && /\.(css|scss)$/.test(node.moduleSpecifier.text)) return undefined;
      return context.factory.updateImportDeclaration(
        node,
        node.modifiers,
        node.importClause,
        rewrite(node.moduleSpecifier),
        node.attributes,
      );
    }
    if (ts.isExportDeclaration(node)) {
      return context.factory.updateExportDeclaration(
        node,
        node.modifiers,
        node.isTypeOnly,
        node.exportClause,
        rewrite(node.moduleSpecifier),
        node.attributes,
      );
    }
    if (ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument)) {
      return context.factory.updateImportTypeNode(
        node,
        context.factory.updateLiteralTypeNode(node.argument, rewrite(node.argument.literal)),
        node.attributes,
        node.qualifier,
        node.typeArguments,
        node.isTypeOf,
      );
    }
    return node;
  };
  return ts.visitNode(sourceFile, visit);
};
rmSync(out, { recursive: true, force: true });
const result = program.emit(undefined, undefined, undefined, true, {
  afterDeclarations: [rewriteAliases],
});
if (result.emitSkipped || result.diagnostics.length) throw new Error("Declaration emission failed");
mkdirSync(dist, { recursive: true });
for (const { entryName, source } of entries) {
  const target = `./_types/${relative(root, source)
    .replaceAll("\\", "/")
    .replace(/\.tsx?$/, ".js")}`;
  const symbol = checker.getSymbolAtLocation(program.getSourceFile(source));
  const hasDefault =
    symbol && checker.getExportsOfModule(symbol).some((item) => item.name === "default");
  writeFileSync(
    join(dist, `${entryName}.d.ts`),
    `export * from ${JSON.stringify(target)};\n${hasDefault ? `export { default } from ${JSON.stringify(target)};\n` : ""}`,
  );
}
