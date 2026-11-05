/* Shared REST client. Passwords and MFA challenges are never persisted. */
"use strict";
const Tayga = (() => {
  const defaults = {
    baseUrl: "",
    email: "",
    token: "",
    refreshToken: "",
    theme: "default",
    ttlHours: 168,
    ewsEnabled: true,
    expiresAt: 0,
  };
  function baseURL(value) {
    const url = new URL(value);
    const local = ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname);
    if (
      (url.protocol !== "https:" && !(url.protocol === "http:" && local)) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      !/^\/*$/.test(url.pathname)
    ) {
      throw new Error(
        "Укажите HTTPS-адрес сервера без пути, параметров и пароля. HTTP разрешён только для localhost.",
      );
    }
    return url.origin;
  }
  function shareURL(value, base) {
    const url = new URL(value, base);
    baseURL(url.origin);
    if (url.username || url.password || !url.pathname.startsWith("/s/"))
      throw new Error("Сервер вернул некорректную ссылку.");
    return url.href;
  }
  function safeFileName(value) {
    const cleanName = (value || "file").replace(
      /[\x00-\x1f\x7f/\\]/g,
      "_",
    );
    let name = "",
      bytes = 0;
    const encoder = new TextEncoder();
    for (const character of cleanName) {
      bytes += encoder.encode(character).length;
      if (bytes > 200) break;
      name += character;
    }
    if (!name || name === "." || name === "..") name = "file";
    return name;
  }
  function waitFor(promise, signal) {
    if (!signal) return promise;
    return new Promise((resolve, reject) => {
      const abort = () => reject(signal.reason);
      if (signal.aborted) { abort(); return; }
      signal.addEventListener("abort", abort, { once: true });
      promise.then(resolve, reject).finally(() => signal.removeEventListener("abort", abort));
    });
  }
  function pause(ms, signal) {
    return new Promise((resolve, reject) => {
      const abort = () => { clearTimeout(timer); reject(signal.reason); };
      const timer = setTimeout(() => {
        signal?.removeEventListener("abort", abort);
        resolve();
      }, ms);
      if (signal?.aborted) abort();
      else signal?.addEventListener("abort", abort, { once: true });
    });
  }
  const transient = (error) => error.name !== "AbortError" &&
    (error.name === "TypeError" || error.name === "TimeoutError" ||
      [408, 409, 429, 502, 503, 504].includes(error.status));
  const retainUpload = (error, signal) => !signal?.aborted &&
    (transient(error) || error.name === "AbortError" || [401, 403].includes(error.status));
  class Client {
    constructor(storage, fetcher = (...args) => fetch(...args), diagnostics = null) {
      this.storage = storage;
      this.diagnostics = diagnostics;
      this.fetcher = fetcher;
      this.refreshing = null;
      this.generation = 0;
      this.uploadClaims = new Set();
    }
    settings() {
      return this.storage.get(defaults);
    }
    async request(base, path, opts = {}) {
      const response = await this.fetcher(baseURL(base) + "/api/v1" + path, {
        ...opts,
        credentials: "omit",
        redirect: "error",
        cache: "no-store",
        signal: opts.signal
          ? AbortSignal.any([opts.signal, AbortSignal.timeout(120000)])
          : AbortSignal.timeout(120000),
      });
      const raw = await response.text();
      let data;
      try {
        data = raw ? JSON.parse(raw) : null;
      } catch {
        if (response.ok) throw new Error(`Некорректный ответ сервера (HTTP ${response.status}).`);
      }
      if (!response.ok) {
        const error = new Error(
          typeof data?.error === "string"
            ? data.error
            : `HTTP ${response.status}`,
        );
        error.status = response.status;
        throw error;
      }
      return data;
    }
    json(body) {
      return {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      };
    }
    async saveTokens(base, email, data, generation) {
      const pair = data.tokens || data;
      if (!pair.access_token || !pair.refresh_token)
        throw new Error("Сервер не выдал токены сессии.");
      if (generation !== this.generation)
        throw new Error("Сессия изменилась. Повторите вход.");
      await this.storage.set({
        baseUrl: base,
        email,
        token: pair.access_token,
        refreshToken: pair.refresh_token,
        expiresAt: Date.now() + (Number(pair.expires_in) || 3600) * 1000,
      });
    }
    async login(base, email, password) {
      base = baseURL(base);
      const generation = ++this.generation;
      const data = await this.request(
        base,
        "/auth/login",
        this.json({ email, password }),
      );
      if (data.mfa_required)
        return {
          mfa: true,
          challenge: data.challenge,
          baseUrl: base,
          email,
          generation,
          methods: data.methods,
        };
      await this.saveTokens(base, email, data, generation);
      return { ok: true };
    }
    async mfa(pending, code) {
      const data = await this.request(
        pending.baseUrl,
        "/auth/mfa/verify",
        this.json({ challenge: pending.challenge, code }),
      );
      await this.saveTokens(
        pending.baseUrl,
        pending.email,
        data,
        pending.generation,
      );
      return { ok: true };
    }
    async logout() {
      ++this.generation;
      await this.storage.remove(["token", "refreshToken"]);
    }
    async refresh(cfg) {
      const current = await this.settings();
      if (
        current.baseUrl !== cfg.baseUrl ||
        current.email !== cfg.email ||
        !current.token
      )
        throw new Error("Сессия изменилась. Повторите операцию.");
      if (current.token !== cfg.token) return current;
      if (!cfg.refreshToken)
        throw new Error("Войдите заново в настройках Tayga.");
      if (!this.refreshing) {
        const generation = this.generation;
        const refresh = async () => {
          const data = await this.request(
            cfg.baseUrl,
            "/auth/token",
            this.json({
              grant_type: "refresh_token",
              refresh_token: cfg.refreshToken,
            }),
          );
          await this.saveTokens(cfg.baseUrl, cfg.email, data, generation);
          return this.settings();
        };
        this.refreshing = (this.diagnostics ? this.diagnostics.observe("auth.refresh", refresh) : refresh()).finally(() => {
          this.refreshing = null;
        });
      }
      return this.refreshing;
    }
    async api(path, opts = {}, cfg) {
      opts.signal?.throwIfAborted();
      cfg ||= await this.settings();
      if (!cfg.token) throw new Error("Сначала войдите в настройках Tayga.");
      // Filelink shares the mail session. Renew before transferring a large
      // body, so an expired bearer never forces the entire file to be resent.
      if (cfg.expiresAt && cfg.expiresAt <= Date.now() + 120000)
        cfg = await waitFor(this.refresh(cfg), opts.signal);
      opts.signal?.throwIfAborted();
      const send = (session) =>
        this.request(session.baseUrl, path, {
          ...opts,
          headers: {
            ...opts.headers,
            Authorization: `Bearer ${session.token}`,
          },
        });
      try {
        return await send(cfg);
      } catch (error) {
        if (error.status !== 401) throw error;
        return send(await waitFor(this.refresh(cfg), opts.signal));
      }
    }
    async pendingUploads(cfg) {
      cfg ||= await this.settings();
      if (!cfg.token) return [];
      const all = await this.storage.get(null);
      return Object.entries(all).filter(([key, value]) =>
        /^pending-upload:[a-f0-9-]{36}$/.test(key) && value?.version === 1 &&
        value.baseUrl === cfg.baseUrl && value.email === cfg.email &&
        /^tb-out\/[a-f0-9-]{36}\/(?!\.\.?$)[^/\\]+$/.test(value.path) &&
        Number.isSafeInteger(value.size) && value.size > 8 * 1024 * 1024 &&
        /^[a-f0-9]{64}$/.test(value.fingerprint) && value.phase !== "delivered"
      ).map(([key, value]) => ({ ...value, key }));
    }
    async rememberUpload(journal) {
      await this.storage.set({ [journal.key]: journal });
    }
    async fileFingerprint(file, signal, report) {
      // Hash in bounded blocks; never load the whole attachment into RAM.
      const hashes = [];
      for (let offset = 0; offset < file.size; offset += 8 * 1024 * 1024) {
        signal?.throwIfAborted();
        const end = Math.min(offset + 8 * 1024 * 1024, file.size);
        const bytes = await file.slice(offset, end).arrayBuffer();
        hashes.push(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)));
        signal?.throwIfAborted();
        report({ phase: "checking", loaded: end, total: file.size });
      }
      const combined = new Uint8Array(hashes.length * 32);
      hashes.forEach((hash, index) => combined.set(hash, index * 32));
      const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", combined));
      return Array.from(digest, (byte) => byte.toString(16).padStart(2, "0")).join("");
    }
    async discardUpload(key) {
      if (this.uploadClaims.has(key)) throw new Error("Сначала отмените текущую загрузку.");
      const cfg = await this.settings();
      const journal = (await this.pendingUploads(cfg)).find((entry) => entry.key === key);
      if (!journal) throw new Error("Загрузка не найдена в текущей учётной записи.");
      // Forget first so a crash during cleanup cannot resume a cancelled job.
      await this.storage.remove(key);
      await this.cleanUploadSession(journal, cfg, true);
    }
    async cleanUploadSession(journal, cfg, discardFile = false) {
      const remove = async (url) => {
        try { await this.api(url, { method: "DELETE", signal: AbortSignal.timeout(5000) }, cfg); }
        catch (error) { if (![404, 410].includes(error.status)) this.diagnostics?.record("upload.cleanup", "failure", { error }); }
      };
      if (journal.uploadId) await remove("/files/uploads/" + encodeURIComponent(journal.uploadId));
      if (discardFile && journal.fileComplete) {
        try {
          const data = await this.api("/files/shares", { signal: AbortSignal.timeout(5000) }, cfg);
          for (const share of data.shares || [])
            if (share.path === journal.path) await remove("/files/shares/" + encodeURIComponent(share.id));
        } catch {}
        await remove("/files?path=" + encodeURIComponent(journal.path));
      }
    }
    async finishUpload(record) {
      if (!record.pendingKey) return;
      const cfg = await this.settings();
      if (cfg.baseUrl !== record.baseUrl || cfg.email !== record.email) return;
      await this.storage.remove(record.pendingKey);
      this.uploadClaims.delete(record.pendingKey);
      await this.cleanUploadSession({ uploadId: record.uploadId }, cfg);
    }
    async upload(file, signal, cfg, displayName = file.name, report = () => {}) {
      cfg ||= await this.settings();
      if (file.size <= 8 * 1024 * 1024)
        return this.uploadFile(file, signal, cfg, displayName, report);
      if (!cfg.token) throw new Error("Сначала войдите в настройках Tayga.");
      const fingerprint = await this.fileFingerprint(file, signal, report);
      signal?.throwIfAborted();
      const name = displayName || file.name || "file";
      const entries = await this.pendingUploads(cfg);
      let journal = entries.find((entry) => entry.name === name && entry.size === file.size &&
        entry.fingerprint === fingerprint && entry.expiresAt > Date.now() && !this.uploadClaims.has(entry.key));
      if (!journal) {
        const id = crypto.randomUUID();
        journal = { key: "pending-upload:" + id, version: 1,
          baseUrl: cfg.baseUrl, email: cfg.email, name, size: file.size, fingerprint,
          path: `tb-out/${id}/${safeFileName(name)}`, uploadId: "", offset: 0,
          expiresAt: Date.now() + 86400000, phase: "uploading" };
      }
      this.uploadClaims.add(journal.key);
      try {
        await this.rememberUpload(journal);
        report({ phase: "uploading", loaded: journal.offset, total: file.size, pendingKey: journal.key });
        return await this.uploadFile(file, signal, cfg, name, report, journal);
      } catch (error) {
        this.uploadClaims.delete(journal.key);
        if (error.expiredUpload) {
          await this.storage.remove(journal.key);
          return this.upload(file, signal, cfg, name, report);
        }
        if (retainUpload(error, signal)) {
          journal.phase = "paused";
          await this.rememberUpload(journal);
        } else {
          await this.storage.remove(journal.key);
          await this.cleanUploadSession(journal, cfg);
        }
        throw error;
      } finally {
        // The Filelink callback acknowledges this lease only after its result
        // is durable; a second attachment must not reuse an in-flight link.
        if (journal.phase !== "ready") this.uploadClaims.delete(journal.key);
      }
    }
    async uploadLarge(file, path, cfg, signal, report, journal) {
      const root = "/files/uploads";
      const request = (url, opts = {}) => this.api(url, { ...opts, signal }, cfg);
      const read = async (url) => {
        for (let attempt = 0; ; attempt++) {
          try { return await request(url); }
          catch (error) {
            if (!transient(error) || signal?.aborted || attempt >= 3) throw error;
            this.diagnostics?.record("upload.retry", "failure", { error });
            await pause((attempt + 1) * 1000, signal);
          }
        }
      };
      let upload;
      if (journal?.uploadId) {
        try { upload = (await read(root + "/" + encodeURIComponent(journal.uploadId))).upload; }
        catch (error) {
          if ([404, 410].includes(error.status)) error.expiredUpload = true;
          throw error;
        }
      } else {
        if (journal?.started) {
          const data = await read(root);
          upload = data.entries?.find((entry) => entry.path === path && entry.size === file.size);
        }
        try {
          if (!upload) {
            if (journal) { journal.started = true; await this.rememberUpload(journal); }
            upload = await request(root, this.json({ path, size: file.size }));
          }
        } catch (error) {
          if (!transient(error) || signal?.aborted) throw error;
          const data = await read(root);
          upload = data.entries?.find((entry) => entry.path === path && entry.size === file.size);
          if (!upload) throw error;
        }
      }
      if (typeof upload?.id !== "string" || !upload.id || upload.path !== path || upload.size !== file.size ||
          !Number.isSafeInteger(upload.offset) || upload.offset < 0 || upload.offset > file.size || (upload.complete && upload.offset !== file.size) ||
          (journal?.uploadId && upload.id !== journal.uploadId))
        throw new Error("Сервер вернул некорректную сессию загрузки.");
      if (journal) {
        journal.uploadId = upload.id;
        journal.offset = upload.offset;
        if (upload.expires && Number.isFinite(Date.parse(upload.expires)))
          journal.expiresAt = Date.parse(upload.expires);
        await this.rememberUpload(journal);
      }
      report({ phase: "uploading", loaded: upload.offset, total: file.size });
      const url = root + "/" + encodeURIComponent(upload.id);
      const validate = (value, min, max) => {
        if (!value || value.id !== upload.id || value.path !== path || value.size !== file.size ||
            !Number.isSafeInteger(value.offset) || value.offset < min || value.offset > max)
          throw new Error("Сервер вернул некорректное смещение загрузки.");
        return value;
      };
      let failures = 0;
      try {
        while (upload.offset < file.size) {
          signal?.throwIfAborted();
          const start = upload.offset;
          const end = Math.min(start + 8 * 1024 * 1024, file.size);
          try {
            const next = await request(url, {
              method: "PATCH",
              headers: { "Content-Type": "application/octet-stream", "Upload-Offset": String(start) },
              body: file.slice(start, end),
            });
            upload = validate(next, end, end);
            failures = 0;
            if (journal) { journal.offset = upload.offset; await this.rememberUpload(journal); }
            report({ phase: "uploading", loaded: upload.offset, total: file.size });
          } catch (error) {
            if (!transient(error) || signal?.aborted || ++failures > 3) throw error;
            this.diagnostics?.record("upload.retry", "failure", { error });
            report({ phase: "retrying", loaded: start, total: file.size, attempt: failures });
            await pause(failures * 1000, signal);
            // Read the durable offset even if the PATCH committed but its response was lost.
            const data = await read(url);
            upload = validate(data.upload, start, end);
            if (upload.offset > start) failures = 0;
            if (journal) { journal.offset = upload.offset; await this.rememberUpload(journal); }
            report({ phase: "uploading", loaded: upload.offset, total: file.size });
          }
        }
        report({ phase: "finishing", loaded: file.size, total: file.size });
        for (let attempt = 0; !upload.complete; attempt++) {
          try {
            upload = validate(await request(url + "/complete", { method: "POST" }), file.size, file.size);
            if (!upload.complete) throw new Error("Сервер не подтвердил сохранение файла.");
            break;
          } catch (error) {
            if (!transient(error) || signal?.aborted || attempt >= 3) throw error;
            this.diagnostics?.record("upload.retry", "failure", { error });
            await pause((attempt + 1) * 1000, signal);
          }
        }
        if (journal) {
          journal.phase = "sharing";
          journal.fileComplete = true;
          await this.rememberUpload(journal);
        }
      } finally {
        if (!journal) {
          try { await this.api(url, { method: "DELETE", signal: AbortSignal.timeout(5000) }, cfg); } catch {}
        }
      }
    }
    async uploadFile(file, signal, cfg, displayName = file.name, report = () => {}, journal) {
      cfg ||= await this.settings();
      const name = safeFileName(displayName);
      const path = journal?.path || `tb-out/${crypto.randomUUID()}/${name}`;
      let uploaded = false;
      report({ phase: "uploading", loaded: 0, total: file.size });
      try {
        if (file.size > 8 * 1024 * 1024) {
          await this.uploadLarge(file, path, cfg, signal, report, journal);
        } else await this.api(
          "/files/content?path=" + encodeURIComponent(path),
          {
            method: "PUT",
            headers: { "Content-Type": "application/octet-stream" },
            body: file,
            signal,
          },
          cfg,
        );
        uploaded = true;
        report({ phase: "sharing", loaded: file.size, total: file.size });
        let share;
        if (journal?.fileComplete) {
          const data = await this.api("/files/shares", { signal }, cfg);
          share = data.shares?.find((entry) => entry.path === path);
        }
        try {
          if (!share) share = await this.api(
            "/files/shares",
            {
              ...this.json({ path, ttl_hours: cfg.ttlHours, rights: "read" }),
              signal,
            },
            cfg,
          );
        } catch (error) {
          if (!transient(error) || signal?.aborted) throw error;
          // The POST may have succeeded. Recover the unique file's link instead
          // of blindly creating another public share after a lost response.
          const data = await this.api("/files/shares", { signal }, cfg);
          share = data.shares?.find((entry) => entry.path === path);
          if (!share) throw error;
        }
        const record = {
          id: share.id,
          path,
          baseUrl: cfg.baseUrl,
          email: cfg.email,
        };
        try {
          if (signal?.aborted) throw new Error("Загрузка отменена.");
          const result = { ...record, url: shareURL(share.url, cfg.baseUrl) };
          if (journal) {
            journal.phase = "ready";
            await this.rememberUpload(journal);
            result.pendingKey = journal.key;
            result.uploadId = journal.uploadId;
          }
          return result;
        } catch (error) {
          await this.api(
            "/files/shares/" + encodeURIComponent(share.id),
            { method: "DELETE", signal: AbortSignal.timeout(5000) },
            cfg,
          );
          throw error;
        }
      } catch (error) {
        if (uploaded && !(journal && retainUpload(error, signal))) {
          try {
            await this.api(
              "/files?path=" + encodeURIComponent(path),
              { method: "DELETE", signal: AbortSignal.timeout(5000) },
              cfg,
            );
          } catch {
            /* Retained in Files for manual cleanup. */
          }
        }
        throw error;
      }
    }
  }
  return { Client, baseURL, shareURL, defaults };
})();
