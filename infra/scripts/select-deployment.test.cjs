const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { selectDeployment } = require('./select-deployment.cjs');
const sha = 'a'.repeat(40);
test('deployment subscribes to the current build and release workflow names', () => {
  const workflowRoot = new URL('../../.github/workflows/', require('node:url').pathToFileURL(__filename));
  const deployment = readFileSync(new URL('deploy-to-server.yaml', workflowRoot), 'utf8');
  const subscriptions = JSON.parse(deployment.match(/^\s+workflows:\s*(\[.*\])/m)[1]);
  const expected = ['build.yml', 'backend-release.yml', 'frontend-release.yml'].map(file =>
    readFileSync(new URL(file, workflowRoot), 'utf8').match(/^name:\s*(.+)$/m)[1].trim());
  assert.deepEqual(subscriptions, expected);
});
function fixture() {
  const run = { id: 123, run_attempt: 1, head_sha: sha, head_branch: 'main', conclusion: 'success',
    event: 'push', path: '.github/workflows/build.yml', head_repository: { full_name: 'org/repo' } };
  const state = { published: true, buildReady: true, taggedSha: sha, prerelease: false };
  const context = { repo: { owner: 'org', repo: 'repo' }, eventName: 'workflow_run', payload: { workflow_run: run } };
  const env = { AUTO_DEPLOY_PRODUCTION_ON_RELEASE: 'true', BUILD_RUN_ID: '123', TARGET_ENVIRONMENT: 'production' };
  const github = { rest: {
    actions: { getWorkflowRun: async () => ({ data: run }), listWorkflowRuns: async () => ({ data: { workflow_runs: state.buildReady ? [run] : [] } }) },
    repos: {
      getContent: async ({ path, ref }) => {
        assert.equal(ref, sha);
        return { data: { content: Buffer.from(path.endsWith('VERSION') ? '1.2.0\n' : '{"version":"1.2.0"}').toString('base64') } };
      },
      getReleaseByTag: async ({ tag }) => {
        assert.match(tag, /^(backend|frontend)\/v1\.2\.0$/);
        if (!state.published || state.missing === tag) throw Object.assign(new Error('not published'), { status: 404 });
        return { data: { draft: false, prerelease: state.prerelease, published_at: '2026-11-04' } };
      }
    },
    git: { getRef: async () => ({ data: { object: { type: 'tag', sha: 'annotated' } } }),
      getTag: async () => ({ data: { object: { type: 'commit', sha: state.taggedSha } } }) }
  } };
  return { run, state, context, env, github };
}
function releaseEvent(f) {
  f.context.payload.workflow_run = { ...f.run, id: 456, head_branch: 'backend/v1.2.0', path: '.github/workflows/backend-release.yml' };
}
test('main builds deploy production only once both releases are published on the build commit', async () => {
  const f = fixture();
  assert.deepEqual(await selectDeployment(f), { run_id: '123', run_attempt: '1', source_commit: sha,
    environment: 'production', automatic: 'true', should_deploy: 'true' });
  for (const missing of ['backend/v1.2.0', 'frontend/v1.2.0']) {
    f.state.missing = missing;
    assert.equal(await selectDeployment(f), null);
  }
});
test('release completion selects the paired main build, never the tag build', async () => {
  const f = fixture(); releaseEvent(f);
  assert.equal((await selectDeployment(f)).run_id, '123');
  f.state.buildReady = false;
  assert.equal(await selectDeployment(f), null);
  f.state.buildReady = true;
  assert.equal((await selectDeployment(f)).environment, 'production');
});
test('main completion after release completion closes the opposite build/release race', async () => {
  const f = fixture(); f.state.published = false;
  assert.equal(await selectDeployment(f), null);
  f.state.published = true;
  assert.equal((await selectDeployment(f)).environment, 'production');
});
test('prereleases and version tags pointing to another commit cannot deploy', async () => {
  const f = fixture(); f.state.prerelease = true;
  assert.equal(await selectDeployment(f), null);
  f.state.prerelease = false; f.state.taggedSha = 'b'.repeat(40);
  assert.equal(await selectDeployment(f), null);
});
test('manual publication is supported alongside token-created release workflow completions', async () => {
  const f = fixture(); f.context.eventName = 'release';
  f.context.payload = { release: { tag_name: 'frontend/v1.2.0', draft: false, prerelease: false } };
  assert.equal((await selectDeployment(f)).run_id, '123');
  f.context.payload.release.prerelease = true;
  assert.equal(await selectDeployment(f), null);
});
test('automatic production is opt-in and staging remains build-only', async () => {
  const f = fixture(); f.env.AUTO_DEPLOY_PRODUCTION_ON_RELEASE = 'false';
  assert.equal(await selectDeployment(f), null);
  f.env.AUTO_DEPLOY_STAGING = 'true';
  assert.equal((await selectDeployment(f)).environment, 'staging');
  releaseEvent(f);
  assert.equal(await selectDeployment(f), null);
});
test('failed, pull request, fork and unrelated workflow events are rejected', async () => {
  for (const change of [{ conclusion: 'failure' }, { event: 'pull_request' },
    { head_repository: { full_name: 'fork/repo' } }, { path: '.github/workflows/other.yml' }]) {
    const f = fixture(); Object.assign(f.run, change);
    assert.equal(await selectDeployment(f), null);
  }
});
test('manual promotion retains provenance validation and optional published-release/no-op mode', async () => {
  const f = fixture(); f.context.eventName = 'workflow_dispatch'; f.state.published = false;
  assert.equal((await selectDeployment(f)).automatic, 'false');
  f.env.RELEASE_ONLY = 'true';
  await assert.rejects(selectDeployment(f), /Both published/);
  f.state.published = true;
  assert.equal((await selectDeployment(f)).automatic, 'true');
  f.run.event = 'pull_request';
  await assert.rejects(selectDeployment(f), /Only successful trusted/);
});
