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
  class Client {
    constructor(storage, fetcher = (...args) => fetch(...args)) {
      this.storage = storage;
      this.fetcher = fetcher;
      this.refreshing = null;
      this.generation = 0;
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
        signal: opts.signal || AbortSignal.timeout(120000),
      });
      const raw = await response.text();
      let data;
      try {
        data = raw ? JSON.parse(raw) : null;
      } catch {
        throw new Error(
          `Некорректный ответ сервера (HTTP ${response.status}).`,
        );
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
        this.refreshing = (async () => {
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
        })().finally(() => {
          this.refreshing = null;
        });
      }
      return this.refreshing;
    }
    async api(path, opts = {}, cfg) {
      cfg ||= await this.settings();
      if (!cfg.token) throw new Error("Сначала войдите в настройках Tayga.");
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
        return send(await this.refresh(cfg));
      }
    }
    async upload(file, signal, cfg, displayName = file.name) {
      cfg ||= await this.settings();
      const cleanName = (displayName || "file").replace(
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
      const path = `tb-out/${crypto.randomUUID()}/${name}`;
      let uploaded = false;
      try {
        await this.api(
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
        const share = await this.api(
          "/files/shares",
          {
            ...this.json({ path, ttl_hours: cfg.ttlHours, rights: "read" }),
            signal,
          },
          cfg,
        );
        const record = {
          id: share.id,
          path,
          baseUrl: cfg.baseUrl,
          email: cfg.email,
        };
        try {
          if (signal?.aborted) throw new Error("Загрузка отменена.");
          return { ...record, url: shareURL(share.url, cfg.baseUrl) };
        } catch (error) {
          await this.api(
            "/files/shares/" + encodeURIComponent(share.id),
            { method: "DELETE" },
            cfg,
          );
          throw error;
        }
      } catch (error) {
        if (uploaded) {
          try {
            await this.api(
              "/files?path=" + encodeURIComponent(path),
              { method: "DELETE" },
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
