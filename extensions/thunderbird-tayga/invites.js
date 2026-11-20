"use strict";
const TaygaInvites = (() => {
  const responses = { accept: "ACCEPTED", tentative: "TENTATIVE", decline: "DECLINED" };
  class Service {
    constructor(client) {
      this.client = client;
      this.saving = false;
    }
    scope(cfg) { return cfg.baseUrl + "\n" + cfg.email.toLowerCase(); }
    async check(scope, generation) {
      const cfg = await this.client.settings();
      if (!cfg.token || this.scope(cfg) !== scope || this.client.generation !== generation)
        throw new Error("Учётная запись изменилась. Обновите список приглашений.");
      return cfg;
    }
    async get(cfg) {
      const data = await this.client.api("/calendar/invites?all=1", {}, cfg);
      if (!Array.isArray(data?.invites)) throw new Error("Сервер не вернул список приглашений.");
      return data.invites;
    }
    async load() {
      const cfg = await this.client.settings();
      if (!cfg.token) return { signedIn: false, invites: [] };
      const scope = this.scope(cfg), generation = this.client.generation;
      const invites = await this.get(cfg);
      await this.check(scope, generation);
      return { signedIn: true, email: cfg.email, scope, generation, invites };
    }
    async reply(request) {
      if (this.saving) throw new Error("Дождитесь сохранения предыдущего ответа.");
      this.saving = true;
      try {
        const status = Object.hasOwn(responses, request.action) ? responses[request.action] : null;
        if (!status || typeof request.id !== "string" || !request.id || request.id.length > 200 ||
            typeof request.etag !== "string" || !request.etag)
          throw new Error("Выберите приглашение и обновите его перед ответом.");
        const cfg = await this.check(request.scope, request.generation);
        const current = (await this.get(cfg)).find((invite) => invite.id === request.id);
        await this.check(request.scope, request.generation);
        if (!current || !current.etag || current.cancelled || current.can_reply === false || current.etag !== request.etag)
          throw new Error("Приглашение изменилось или отменено. Обновите список перед ответом.");
        if (current.partstat === status && !current.proposed_start)
          return { already: true, partstat: status };
        // No automatic network replay: after an uncertain result the user must
        // reload. The fresh status then prevents sending the same reply twice.
        const result = await this.client.api("/calendar/invites/" + encodeURIComponent(request.id) + "/reply",
          this.client.json({ action: request.action, etag: current.etag }), cfg);
        await this.check(request.scope, request.generation);
        if (result?.partstat !== status) throw new Error("Сервер не подтвердил ответ. Обновите список приглашений.");
        return { partstat: result.partstat };
      } finally { this.saving = false; }
    }
  }
  return { Service };
})();
if (typeof module !== "undefined") module.exports = TaygaInvites;
