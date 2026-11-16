import type { Plugin } from "vite";
export interface ModuleSelection {
  environment: string;
  source: "workspace" | "npm";
  versions: Record<string, string>;
  directory: string;
  modules: Array<{ group: string; name: string; pkg: { name: string; version: string } }>;
}
export function moduleSelection(
  environment?: string,
  env?: Record<string, string | undefined>,
  root?: string,
): ModuleSelection;
export function npmModulePlugin(selection: ModuleSelection): Plugin;
