// Injected ONLY into a temporary smoke-test copy of the Experiment.
async function taygaNativeTest(api, args) {
  if (args.op === "capabilities")
    return {
      interfaces: Object.keys(Ci).filter((k) => /ews|outgoing/i.test(k)),
      factories: Object.keys(Cc).filter((k) => /ews/.test(k)),
    };
  if (args.op === "removeManagedMail") {
    const record = Object.values(api.state.mail)[0];
    const account = Array.from(MailServices.accounts.accounts).find(
      (a) => a.key === record.accountId,
    );
    MailServices.accounts.removeAccount(account, false);
    return { ok: true };
  }
  if (args.op === "mail") {
    const record = Object.values(api.state.mail)[0];
    const server = MailServices.accounts.getIncomingServer(record.serverKey);
    const folders = Array.from(server.rootFolder.descendants);
    const inbox = folders.find((f) => f.flags & Ci.nsMsgFolderFlags.Inbox);
    if (inbox && args.markRead)
      inbox.markMessagesRead(Array.from(inbox.messages), true);
    if (inbox && args.update) inbox.updateFolder(null);
    return {
      folders: folders.map((f) => ({
        name: f.name,
        parent: f.parent?.name,
        count: f.getTotalMessages(false),
      })),
      inbox: inbox
        ? Array.from(inbox.messages).map((m) => ({
            subject: m.subject,
            date: m.dateInSeconds,
            key: m.messageKey,
          }))
        : [],
    };
  }
  const collection = api.state.collections[args.key];
  const target = api.native(collection);
  if (collection.kind !== "contacts") {
    let item;
    if (args.data) {
      const parser = Cc["@mozilla.org/calendar/ics-parser;1"].createInstance(
        Ci.calIIcsParser,
      );
      parser.parseString(args.data);
      item = parser.getItems()[0];
      item.calendar = target;
    } else item = await target.getItem(args.localId);
    if (args.op === "get") {
      const serializer = Cc[
        "@mozilla.org/calendar/ics-serializer;1"
      ].createInstance(Ci.calIIcsSerializer);
      if (item) serializer.addItems([item]);
      return { data: item ? serializer.serializeToString() : "" };
    }
    if (args.op === "create" || args.op === "legacyTask")
      await target.addItem(item);
    if (args.op === "legacyTask") {
      api.queue(collection, item.id, "put", args.data, item.title);
      const p = api.state.pending[collection.key + ":" + item.id];
      p.error =
        "EWS CreateItem: ErrorInvalidPropertySet Tayga sync write rejected";
      p.conflict = true;
      p.attempted = true;
      await api.save();
    }
    if (args.op === "unobservedModify") {
      const { observer } = api.calObservers.get(collection.key);
      target.removeObserver(observer);
      try {
        const changed = item.clone();
        if (args.title !== undefined) changed.title = args.title;
        if (args.status !== undefined) changed.status = args.status;
        if (args.percent !== undefined) changed.percentComplete = args.percent;
        await target.modifyItem(changed, item);
      } finally {
        target.addObserver(observer);
      }
    }
    if (args.op === "modify") {
      const changed = item.clone();
      if (args.title !== undefined) changed.title = args.title;
      if (args.status !== undefined) changed.status = args.status;
      if (args.percent !== undefined) changed.percentComplete = args.percent;
      await target.modifyItem(changed, item);
    }
    if (args.op === "delete") await target.deleteItem(item);
  } else {
    let card = args.data
      ? VCardUtils.vCardToAbCard(args.data)
      : target.getCard(args.localId);
    if (!args.data && !target.hasCard(card)) card = null;
    if (args.op === "get")
      return { data: card ? VCardUtils.abCardToVCard(card, "4.0") : "" };
    if (args.op === "create") target.addCard(card);
    if (args.op === "modify") {
      card.displayName = args.title;
      target.modifyCard(card);
    }
    if (args.op === "delete") target.deleteCards([card]);
  }
  return { ok: true };
}
