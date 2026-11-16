import { readFileSync } from "node:fs";
import { join } from "node:path";
import { check, fileExists, finish, readJson, root } from "./workspace-lib.mjs";

const failures = [];
const packageJson = readJson(join(root, "package.json"));
const dockerfile = readFileSync(join(root, "../infra/docker/frontend/Dockerfile"), "utf8");
const npmDockerfilePath = join(root, "../infra/docker/frontend/Dockerfile.npm-modules");
const npmDockerfile = fileExists(npmDockerfilePath) ? readFileSync(npmDockerfilePath, "utf8") : "";
const imports = readJson(join(root, "public", "import-map.json")).imports ?? {};

check(
  Boolean(packageJson.scripts?.["build:docker"]),
  "package.json is missing build:docker",
  failures,
);
check(
  Boolean(packageJson.scripts?.["build:docker:npm-modules"]),
  "package.json is missing build:docker:npm-modules",
  failures,
);
check(Boolean(packageJson.scripts?.["stage:mf"]), "package.json is missing stage:mf", failures);
check(
  Boolean(packageJson.scripts?.["stage:mf:npm"]),
  "package.json is missing stage:mf:npm",
  failures,
);
check(
  dockerfile.includes("use-runtime-config.mjs production"),
  "Dockerfile should switch to production runtime config",
  failures,
);
check(dockerfile.includes("npm run build:docker"), "Dockerfile should run build:docker", failures);
const npmBuild = packageJson.scripts?.["build:docker:npm-prepared"] || "";
check(
  npmDockerfile.includes("ARG FRONTEND_MODULE_SOURCE=npm") &&
    npmDockerfile.includes("npm run build:docker") &&
    npmBuild.includes("npm run build:shell") &&
    npmBuild.includes("npm run generate:import-map") &&
    npmBuild.includes("npm run stage:mf:npm") &&
    npmBuild.includes("npm run audit:import-map") &&
    npmBuild.indexOf("generate:import-map") < npmBuild.indexOf("build:shell"),
  "Dockerfile.npm-modules should select npm modules and generate metadata before building/staging the shell",
  failures,
);

for (const specifier of [
  "react",
  "react-dom",
  "single-spa",
  "@moh-sso/users",
  "@moh-sso/api",
  "@moh-sso/ui",
]) {
  check(Boolean(imports[specifier]), `import map is missing ${specifier}`, failures);
}

finish("Deployment doctor", failures);
