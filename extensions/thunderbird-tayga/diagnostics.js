"use strict";
const TaygaDiagnostics = (() => {
  const key = "diagnostics:v1";
  const events = new Set(["startup", "connection.available", "auth.login", "auth.mfa", "auth.logout", "auth.refresh", "sync", "upload", "upload.phase", "upload.cleanup", "upload.retry", "invite.reply", "rules.save", "session.configure", "theme", "filelink.configure"]);
  const results = new Set(["start", "success", "failure", "cancelled", "mfa_required"]);
  const phases = new Set(["checking", "uploading", "retrying", "finishing", "sharing", "complete", "cancelled", "failed", "paused"]);
  const categories = new Set(["network", "timeout", "cancelled", "auth", "conflict", "server", "other"]);
  const protocolCodes = new Set(["ErrorIrresolvableConflict", "ErrorInvalidSyncStateData", "ErrorAccessDenied", "ErrorItemNotFound", "ErrorChangeKeyRequired", "ErrorInvalidPropertySet", "ErrorInternalServerError"]);
  function errorFields(error) {
    const status = error?.status;
    const fields = {};
    if (Number.isInteger(status) && status >= 400 && status <= 599) fields.httpStatus = status;
    if (protocolCodes.has(error?.code)) fields.protocolCode = error.code;
    fields.errorCategory = status === 401 || status === 403 || error?.code === "ErrorAccessDenied" ? "auth"
      : status === 409 || error?.code === "ErrorIrresolvableConflict" ? "conflict"
      : error?.name === "AbortError" ? "cancelled"
      : error?.name === "TimeoutError" ? "timeout"
      : error?.name === "TypeError" || error?.name === "NetworkError" ? "network"
      : status >= 500 && status <= 599 ? "server" : "other";
    return fields;
  }
  function row(event, result, details, time) {
    if (!events.has(event) || !results.has(result) || !Number.isFinite(time)) return null;
    const entry = { time: new Date(time).toISOString(), event, result };
    if (details.error) Object.assign(entry, errorFields(details.error));
    // Deliberately copy only enumerated metadata. Never copy errors, messages,
    // stacks, request objects, URLs, account IDs, subjects or filenames.
    if (categories.has(details.errorCategory)) entry.errorCategory = details.errorCategory;
    if (protocolCodes.has(details.protocolCode)) entry.protocolCode = details.protocolCode;
    if (Number.isInteger(details.httpStatus) && details.httpStatus >= 400 && details.httpStatus <= 599) entry.httpStatus = details.httpStatus;
    if (Number.isFinite(details.durationMs) && details.durationMs >= 0 && details.durationMs <= 86400000) entry.durationMs = Math.round(details.durationMs);
    if (phases.has(details.phase)) entry.phase = details.phase;
    if (["wake", "online"].includes(details.reason)) entry.reason = details.reason;
    return entry;
  }
  class Journal {
    constructor(storage, { now = () => Date.now(), limit = 500 } = {}) {
      this.storage = storage;
      this.now = now;
      this.limit = limit;
      this.ttl = 7 * 86400000;
      this.enabled = true;
      this.healthy = true;
      this.entries = [];
      this.tail = Promise.resolve();
      this.ready = this.initialize();
    }
    async initialize() {
      try {
        const data = (await this.storage.get(key))[key];
        this.enabled = data?.enabled !== false;
        this.entries = (Array.isArray(data?.entries) ? data.entries : [])
          .map((value) => value && row(value.event, value.result, value, typeof value.time === "string" ? Date.parse(value.time) : NaN)).filter(Boolean);
        this.prune();
        await this.persist();
      } catch { this.healthy = false; }
    }
    prune() {
      const now = this.now();
      this.entries = this.entries.filter((entry) => { const time = Date.parse(entry.time); return time <= now && time >= now - this.ttl; }).slice(-this.limit);
    }
    async persist() {
      await this.storage.set({ [key]: { enabled: this.enabled, entries: this.entries } });
      this.healthy = true;
    }
    queue(work) {
      const next = this.tail.then(() => this.ready).then(work);
      this.tail = next.catch(() => {});
      return next;
    }
    record(event, result, details = {}) {
      // Sanitize immediately, before asynchronous work can retain sensitive
      // objects in a queued closure.
      let entry;
      try { entry = row(event, result, details, this.now()); } catch { return Promise.resolve(); }
      if (!entry) return Promise.resolve();
      return this.queue(async () => {
        if (!this.enabled) return;
        this.entries.push(entry); this.prune();
        await this.persist();
      }).catch(() => { this.healthy = false; });
    }
    async observe(event, work) {
      const started = this.now();
      this.record(event, "start");
      try {
        const value = await work();
        this.record(event, "success", { durationMs: this.now() - started });
        return value;
      } catch (error) {
        this.record(event, error?.name === "AbortError" ? "cancelled" : "failure", { error, durationMs: this.now() - started });
        throw error;
      }
    }
    snapshot() { return { enabled: this.enabled, healthy: this.healthy, limit: this.limit, maxAgeDays: 7, entries: this.entries.map((value) => ({ ...value })) }; }
    view() {
      return this.queue(async () => {
        const count = this.entries.length; this.prune();
        if (count !== this.entries.length) { try { await this.persist(); } catch { this.healthy = false; } }
        return this.snapshot();
      });
    }
    setEnabled(enabled) {
      if (typeof enabled !== "boolean") return Promise.reject(new Error("Укажите состояние записи журнала."));
      return this.queue(async () => { this.enabled = enabled; await this.persist(); return this.snapshot(); });
    }
    clear() {
      return this.queue(async () => { this.entries = []; await this.persist(); return this.snapshot(); });
    }
    async export(version, browserVersion) {
      const data = await this.view();
      return { schema: "tayga-diagnostics/v1", pluginVersion: typeof version === "string" && /^\d+\.\d+\.\d+$/.test(version) ? version : "unknown",
        thunderbirdVersion: typeof browserVersion === "string" && /^\d+(?:\.\d+){0,3}(?:[ab]\d+)?$/.test(browserVersion) ? browserVersion : "unknown",
        generatedAt: new Date(this.now()).toISOString(), ...data };
    }
  }
  return { Journal };
})();
if (typeof module !== "undefined") module.exports = TaygaDiagnostics;
