import { execFileSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  realpathSync,
  writeFileSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const projectRoot = fileURLToPath(new URL("..", import.meta.url));
const read = (path) => (existsSync(path) ? readFileSync(path, "utf8") : "");
const escape = (text) => text.replace(/[\\`*_[\]<>]/g, "\\$&");

export function changelogEntry(contents, version) {
  const headings = [...contents.matchAll(/^## ([^\n]+)$/gm)];
  const index = headings.findIndex((match) => match[1].trim() === version);
  if (index < 0) return "";
  const start = headings[index].index + headings[index][0].length;
  const end = headings[index + 1]?.index ?? contents.length;
  return contents
    .slice(start, end)
    .split("<!-- scoped-source-changes -->")[0]
    .trim();
}

export function updateChangelog(contents, version, body) {
  const headings = [...contents.matchAll(/^## ([^\n]+)$/gm)];
  const index = headings.findIndex((match) => match[1].trim() === version);
  const entry = `## ${version}\n\n${body.trim()}\n\n`;
  if (index >= 0) {
    return (
      contents.slice(0, headings[index].index) +
      entry +
      contents.slice(headings[index + 1]?.index ?? contents.length)
    );
  }
  const first = headings[0]?.index;
  if (first !== undefined)
    return contents.slice(0, first) + entry + contents.slice(first);
  return `${contents.trim() || "# Changelog"}\n\n${entry}`;
}

export function generateReleaseNotes({
  root = projectRoot,
  component = "both",
  backendVersion,
  sourceCommit = "HEAD",
  output = join(root, ".release/notes"),
  writeChangelogs = true,
  run = execFileSync,
}) {
  if (!["both", "frontend", "backend"].includes(component))
    throw new Error("Invalid component for release notes");
  const git = (...args) =>
    run("git", args, { cwd: root, encoding: "utf8" }).trim();
  const commit = git("rev-parse", sourceCommit);
  const files = [];
  const write = (path, contents) => {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, contents);
    files.push(path);
  };
  const previousTag = (name, version) =>
    git(
      "tag",
      "--merged",
      commit,
      "--list",
      `${name}/v*`,
      "--sort=-version:refname",
    )
      .split("\n")
      .find((tag) => tag && tag !== `${name}/v${version}`);
  const commits = (previous, paths) =>
    git(
      "log",
      "--format=%h%x09%s",
      previous ? `${previous}..${commit}` : commit,
      "--",
      ...paths,
    )
      .split("\n")
      .filter(
        (line) => line && !line.split("\t")[1]?.startsWith("Prepare release:"),
      );
  const sourceChanges = (previous, changes) =>
    `<!-- scoped-source-changes -->\n### Source changes\n\n${previous ? `Changes since \`${previous}\`.` : "Initial release; includes available source history."}\n\n${
      changes.length
        ? changes
            .map((line) => {
              const [sha, ...subject] = line.split("\t");
              return `- ${escape(subject.join("\t"))} (\`${sha}\`)`;
            })
            .join("\n")
        : "- No direct source changes; this release updates version or dependency metadata."
    }`;
  const metadata = { sourceCommit: commit, components: {} };

  if (component !== "frontend") {
    const version =
      backendVersion || read(join(root, "backend/VERSION")).trim();
    if (
      !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/.test(
        version,
      )
    )
      throw new Error("Invalid backend version for release notes");
    const previous = previousTag("backend", version);
    const path = join(root, "backend/CHANGELOG.md");
    const body = [
      changelogEntry(read(path), version),
      sourceChanges(previous, commits(previous, ["backend"])),
    ]
      .filter(Boolean)
      .join("\n\n");
    const notes = `# Backend ${version}\n\n${body}\n`;
    write(join(output, "backend.md"), notes);
    write(join(output, "backend", `${version}.md`), notes);
    if (writeChangelogs)
      write(path, updateChangelog(read(path), version, body));
    metadata.components.backend = { version, previousTag: previous ?? null };
  }

  if (component !== "backend") {
    const shell = JSON.parse(
      read(join(root, "frontend/apps/shell/package.json")),
    );
    const previous = previousTag("frontend", shell.version);
    const modules = [];
    for (const group of ["apps", "packages"]) {
      for (const entry of readdirSync(join(root, "frontend", group), {
        withFileTypes: true,
      }).sort((a, b) => a.name.localeCompare(b.name))) {
        const folder = join(root, "frontend", group, entry.name);
        if (!entry.isDirectory() || !existsSync(join(folder, "package.json")))
          continue;
        const pkg = JSON.parse(read(join(folder, "package.json")));
        if (
          !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/.test(
            pkg.version,
          )
        )
          throw new Error(`Invalid module version: ${pkg.name}`);
        const path = join(folder, "CHANGELOG.md");
        const body = [
          changelogEntry(read(path), pkg.version),
          sourceChanges(
            previous,
            commits(previous, [`frontend/${group}/${entry.name}`]),
          ),
        ]
          .filter(Boolean)
          .join("\n\n");
        write(
          join(output, "frontend", group, entry.name, `${pkg.version}.md`),
          `# ${pkg.name} ${pkg.version}\n\n${body}\n`,
        );
        if (writeChangelogs)
          write(path, updateChangelog(read(path), pkg.version, body));
        modules.push({ name: pkg.name, version: pkg.version, body });
      }
    }
    const notes = `# Frontend ${shell.version}\n\n${modules.map((pkg) => `## ${pkg.name} ${pkg.version}\n\n${pkg.body}`).join("\n\n")}\n`;
    write(join(output, "frontend.md"), notes);
    metadata.components.frontend = {
      version: shell.version,
      previousTag: previous ?? null,
      modules: modules.map(({ name, version }) => ({ name, version })),
    };
  }
  write(
    join(output, "manifest.json"),
    `${JSON.stringify(metadata, null, 2)}\n`,
  );
  return { files, metadata };
}

if (
  process.argv[1] &&
  realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const [component = "both", backendVersion, sourceCommit = "HEAD", output] =
      process.argv.slice(2);
    const result = generateReleaseNotes({
      component,
      backendVersion: backendVersion || undefined,
      sourceCommit,
      output,
    });
    console.log(
      `Generated ${result.files.length} release-note/changelog files.`,
    );
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
