import Database from "better-sqlite3";

export class DB {
  constructor(dbPath) {
    this.dbPath = dbPath;
    this.db = null;
  }

  init() {
    this.db = new Database(this.dbPath);
    this.db.pragma("journal_mode = WAL");
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS users (
        chat_id INTEGER PRIMARY KEY,
        group_name TEXT,
        week_mode TEXT
      );
    `);
  }

  getUser(chatId) {
    const row = this.db
      .prepare("SELECT group_name, week_mode FROM users WHERE chat_id=?")
      .get(chatId);

    if (!row) return { groupName: null, weekMode: "auto" };

    const weekMode = ["auto", "numerator", "denominator"].includes(row.week_mode)
      ? row.week_mode
      : "auto";

    return { groupName: row.group_name || null, weekMode };
  }

  setUserGroup(chatId, groupName) {
    this.db.prepare(`
      INSERT INTO users(chat_id, group_name, week_mode)
      VALUES(@chat_id, @group_name, COALESCE((SELECT week_mode FROM users WHERE chat_id=@chat_id), 'auto'))
      ON CONFLICT(chat_id) DO UPDATE SET group_name=excluded.group_name
    `).run({ chat_id: chatId, group_name: groupName });
  }

  setUserWeekMode(chatId, weekMode) {
    this.db.prepare(`
      INSERT INTO users(chat_id, group_name, week_mode)
      VALUES(@chat_id, COALESCE((SELECT group_name FROM users WHERE chat_id=@chat_id), NULL), @week_mode)
      ON CONFLICT(chat_id) DO UPDATE SET week_mode=excluded.week_mode
    `).run({ chat_id: chatId, week_mode: weekMode });
  }
}