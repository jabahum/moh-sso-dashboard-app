import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { URL } from "node:url";
import { createRequire } from "node:module";
import test from "node:test";
import { runInNewContext } from "node:vm";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

const require = createRequire(import.meta.url);
function load(path, dependencies = {}) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
      jsx: ts.JsxEmit.ReactJSX,
    },
  });
  const exports = {};
  runInNewContext(outputText, {
    exports,
    require: (name) => (name in dependencies ? dependencies[name] : require(name)),
  });
  return exports;
}
const auth = load("../packages/auth/src/auth/permissions.ts");
const policy = load("../apps/report-scheduler/src/utils/scheduler-permissions.ts", {
  "@moh-sso/auth": auth,
});
const read = "report_scheduler:read";
const accessFor = (permissions, system = true) =>
  policy.schedulerPermissions((name) => permissions.includes(name), system);
const actions = ["create", "update", "delete", "execute", "history", "manage"];

test("each scheduler permission requires read and parent system access", () => {
  const all = [read, ...actions.map((name) => `report_scheduler:${name}`)];
  for (const [permissions, system] of [
    [all, false],
    [all.slice(1), true],
    [[], true],
  ]) {
    assert.ok(Object.values(accessFor(permissions, system)).every((value) => !value));
  }
  for (const action of actions) {
    const access = accessFor([read, `report_scheduler:${action}`]);
    assert.equal(access.read, true);
    for (const candidate of actions) assert.equal(access[candidate], candidate === action);
  }
});

test("management scope cannot substitute for mutation permissions", () => {
  const manager = accessFor([read, "report_scheduler:manage"]);
  for (const action of ["run", "pause", "resume", "duplicate", "delete", "unknown"])
    assert.equal(policy.canScheduleAction(manager, action), false);
  for (const [permission, allowed] of [
    ["create", ["duplicate"]],
    ["update", ["pause", "resume"]],
    ["execute", ["run"]],
    ["delete", ["delete"]],
  ]) {
    const access = accessFor([read, `report_scheduler:${permission}`]);
    for (const action of ["run", "pause", "resume", "duplicate", "delete"])
      assert.equal(policy.canScheduleAction(access, action), allowed.includes(action));
  }
});

// Render real scheduler components with lightweight Carbon controls and API
// hooks. This checks both visible actions and query subscriptions without a DOM.
const control = ({ children, itemText }) => React.createElement("div", null, children, itemText);
const carbon = new Proxy(
  {},
  {
    get: (_, name) => {
      if (name === "TabList" || name === "TabPanels")
        return ({ children }) => {
          assert.equal(
            React.Children.count(children),
            currentAccess.history ? 3 : 2,
            "tab and panel indices must remain contiguous without empty permission slots",
          );
          return React.createElement("div", null, children);
        };
      return control;
    },
  },
);
let currentAccess;
let queries;
const schedule = {
  id: "schedule",
  reportName: "Monthly report",
  healthBiReportId: "report",
  frequency: "monthly",
  periodStrategy: "previous_month",
  outputFormat: "pdf",
  timezone: "Africa/Kampala",
  enabled: true,
};
const api = new Proxy(
  {},
  {
    get:
      (_, name) =>
      (...args) => {
        if (!String(name).startsWith("useGet"))
          return [() => ({ unwrap: async () => ({}) }), { isLoading: false }];
        const skip = !!args[1]?.skip;
        queries.push({ name, skip });
        let data;
        if (!skip) {
          if (name === "useGetReportSchedulerModuleQuery") data = { healthBiEnabled: true };
          else if (name === "useGetReportSchedulerOverviewQuery") data = { workerHealthy: true };
          else if (name === "useGetReportSchedulesQuery") data = [schedule];
          else data = [];
        }
        return {
          data,
          currentData: data,
          isLoading: false,
          isFetching: false,
          isError: false,
          refetch: () => {},
        };
      },
  },
);
const ui = {
  useHeaderPanel: () => ({ openPanel: () => {}, closePanel: () => {} }),
  useToast: () => ({ error: () => {}, success: () => {} }),
};
const permissionHook = { useSchedulerPermissions: () => currentAccess };
const tables = load("../apps/report-scheduler/src/components/SchedulerTables.tsx", {
  "@carbon/react": carbon,
});
const root = load("../apps/report-scheduler/src/root.component.tsx", {
  "@carbon/react": carbon,
  "@moh-sso/ui": ui,
  "./api": api,
  "./hooks/useSchedulerPermissions": permissionHook,
  "./hooks/useSchedulerQueryToasts": { useSchedulerQueryToasts: () => {} },
  "./utils/scheduler-permissions": policy,
  "./components/SchedulerTables": tables,
  "./components/ScheduleFormPanel": { ScheduleFormPanel: control },
  "./report-scheduler.scss": {},
});
function render(permissions, system = true) {
  queries = [];
  currentAccess = accessFor(permissions, system);
  return renderToStaticMarkup(React.createElement(root.ReportSchedulerRoot, {}));
}

test("unauthorized entry mounts no API queries", () => {
  for (const [permissions, system] of [
    [[], true],
    [[read], false],
  ]) {
    assert.match(render(permissions, system), /do not have permission/);
    assert.equal(queries.length, 0);
  }
});
test("read-only users see schedules and inbox without management actions or history requests", () => {
  const html = render([read]);
  for (const label of ["Scheduled reports", "Delivered to me"]) assert.ok(html.includes(label));
  for (const label of [
    "Create schedule",
    "Run now",
    "Edit schedule",
    "Pause schedule",
    "Duplicate schedule",
    "Delete schedule",
    "Execution history",
    "Scheduler health",
  ])
    assert.ok(!html.includes(label), label);
  for (const name of [
    "useGetReportSchedulerOverviewQuery",
    "useGetReportExecutionsQuery",
    "useGetReportExecutionQuery",
    "useGetHealthBIReportsQuery",
  ])
    assert.equal(queries.find((query) => query.name === name)?.skip, true, name);
  assert.equal(queries.find((query) => query.name === "useGetPortalReportsQuery")?.skip, false);
});
test("action controls are independently gated while an executor can run without history", () => {
  for (const [permission, visible, absent] of [
    [
      "create",
      ["Create schedule", "Duplicate schedule"],
      ["Edit schedule", "Delete schedule", "Run now"],
    ],
    [
      "update",
      ["Edit schedule", "Pause schedule"],
      ["Create schedule", "Duplicate schedule", "Delete schedule", "Run now"],
    ],
    ["delete", ["Delete schedule"], ["Edit schedule", "Create schedule", "Run now"]],
    [
      "execute",
      ["Run now"],
      ["Create schedule", "Edit schedule", "Delete schedule", "Execution history"],
    ],
    [
      "history",
      ["Execution history", "Scheduler health"],
      ["Run now", "Create schedule", "Edit schedule", "Delete schedule"],
    ],
  ]) {
    const html = render([read, `report_scheduler:${permission}`]);
    for (const label of visible) assert.ok(html.includes(label), `${permission}: ${label}`);
    for (const label of absent) assert.ok(!html.includes(label), `${permission}: ${label}`);
  }
});

test("retry remains unavailable to a history-only viewer and detached executions", () => {
  const row = {
    id: "execution",
    scheduleId: "schedule",
    reportId: "report",
    status: "failed",
    triggerType: "manual",
    outputFormat: "pdf",
    generationAttempts: 1,
    maxGenerationAttempts: 3,
  };
  const props = {
    rows: [row],
    schedules: [],
    onDetails: () => {},
    onRetry: () => {},
    retrying: false,
    expandedDetails: null,
  };
  assert.ok(
    !renderToStaticMarkup(
      React.createElement(tables.ExecutionsTable, { ...props, canRetry: false }),
    ).includes("Retry"),
  );
  assert.ok(
    renderToStaticMarkup(
      React.createElement(tables.ExecutionsTable, { ...props, canRetry: true }),
    ).includes("Retry"),
  );
  assert.ok(
    !renderToStaticMarkup(
      React.createElement(tables.ExecutionsTable, {
        ...props,
        canRetry: true,
        rows: [{ ...row, scheduleId: undefined }],
      }),
    ).includes("Retry"),
  );
});

const form = load("../apps/report-scheduler/src/components/ScheduleFormPanel.tsx", {
  "@carbon/react": carbon,
  "@moh-sso/ui": ui,
  "../api": api,
  "../hooks/useSchedulerPermissions": permissionHook,
  "../hooks/useSchedulerQueryToasts": { useSchedulerQueryToasts: () => {} },
  "../utils/report-parameters": load("../apps/report-scheduler/src/utils/report-parameters.ts"),
  "./RecipientSelect": { RecipientSelect: control },
  "../report-scheduler.scss": {},
});
test("an independently mounted form requires the correct create or update permission", () => {
  for (const [permissions, initialSchedule] of [
    [[read], undefined],
    [[read, "report_scheduler:create"], schedule],
    [[read, "report_scheduler:update"], undefined],
    [[read, "report_scheduler:manage"], schedule],
  ]) {
    currentAccess = accessFor(permissions);
    queries = [];
    const html = renderToStaticMarkup(
      React.createElement(form.ScheduleFormPanel, {
        initialSchedule,
        currentUserId: "user",
        onCancel: () => {},
        onSuccess: () => {},
      }),
    );
    assert.ok(html.includes("do not have permission"));
    assert.equal(queries.length, 0, "denied form must not fetch recipient or report data");
  }
});
