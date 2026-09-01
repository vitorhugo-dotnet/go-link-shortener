import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import vm from 'node:vm';

async function loadTools(fetchImpl) {
  const registrations = [];
  const listeners = new Map();
  const document = {
    readyState: 'complete',
    addEventListener(type, listener) { listeners.set(type, listener); },
    modelContext: { registerTool: async (tool) => registrations.push(tool) },
  };
  const source = await readFile(new URL('./webmcp.js', import.meta.url), 'utf8');
  vm.runInNewContext(source, { document, fetch: fetchImpl, encodeURIComponent, JSON, Error });

  // registerTools awaits each registration; let both promise continuations run.
  await new Promise(setImmediate);
  await new Promise(setImmediate);
  return registrations;
}

test('registers the two read-only link tools', async () => {
  const tools = await loadTools(async () => ({ ok: true, json: async () => ({}) }));
  assert.deepEqual(tools.map((tool) => tool.name), ['resolve_short_link', 'get_link_analytics']);
  for (const tool of tools) {
    assert.equal(tool.annotations.readOnlyHint, true);
    assert.deepEqual(Array.from(tool.inputSchema.required), ['alias']);
  }
});

test('resolve tool encodes aliases and returns the API payload', async () => {
  let requestedURL;
  const tools = await loadTools(async (url) => {
    requestedURL = url;
    return { ok: true, json: async () => ({ alias: 'hello-world', shortUrl: 'http://short/hello-world', targetUrl: 'https://example.com' }) };
  });
  const result = await tools[0].execute({ alias: 'hello world' });
  assert.equal(requestedURL, '/api/links/hello%20world');
  assert.equal(result, '{"alias":"hello-world","shortUrl":"http://short/hello-world","targetUrl":"https://example.com"}');
});

test('analytics tool requests the aggregate endpoint for the encoded alias', async () => {
  let requestedURL;
  const tools = await loadTools(async (url) => {
    requestedURL = url;
    return { ok: true, json: async () => ({ alias: 'hello-world', totalClicks: 42, lastClickedAt: '2026-09-01T14:20:00Z' }) };
  });
  const result = await tools[1].execute({ alias: 'hello world' });
  assert.equal(requestedURL, '/api/links/hello%20world/analytics');
  assert.equal(result, '{"alias":"hello-world","totalClicks":42,"lastClickedAt":"2026-09-01T14:20:00Z"}');
});
