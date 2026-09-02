(function () {
  async function fetchToolResult(path, alias, suffix, signal) {
    const response = await fetch(path + encodeURIComponent(alias) + suffix, { signal: signal });
    const payload = await response.json().catch(function () { return {}; });
    if (!response.ok) {
      throw new Error(payload.error || "Unable to complete the link request.");
    }
    return JSON.stringify(payload);
  }

  async function registerTools() {
    if (!document.modelContext || typeof document.modelContext.registerTool !== "function") {
      return;
    }

    const inputSchema = {
      type: "object",
      properties: {
        alias: { type: "string", description: "The short-link alias to look up." }
      },
      required: ["alias"]
    };

    await document.modelContext.registerTool({
      name: "resolve_short_link",
      description: "Look up a short link and return its short URL and destination URL. Use this when a user asks where an alias redirects.",
      inputSchema: inputSchema,
      annotations: { readOnlyHint: true },
      execute: function (input, context) {
        return fetchToolResult("/api/links/", input.alias, "", context && context.signal);
      }
    });

    await document.modelContext.registerTool({
      name: "get_link_analytics",
      description: "Get aggregate click metrics for a short-link alias. Use this when a user asks about link traffic.",
      inputSchema: inputSchema,
      annotations: { readOnlyHint: true },
      execute: function (input, context) {
        return fetchToolResult("/api/links/", input.alias, "/analytics", context && context.signal);
      }
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", registerTools, { once: true });
  } else {
    registerTools();
  }
}());
