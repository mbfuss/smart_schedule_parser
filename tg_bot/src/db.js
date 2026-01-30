import Database from "better-sqlite3";
import { normText, safeJsonParse, uniqRecent } from "./util.js";

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

    // --- migrations: add columns if missing ---
    const cols = this.db.prepare("PRAGMA table_info(users)").all().map((r) => r.name);

    const addCol = (name, ddl) => {
      if (cols.includes(name)) return;
      this.db.exec(`ALTER TABLE users ADD COLUMN ${ddl};`);
    };

    addCol("recent_teachers", "recent_teachers TEXT");
    addCol("recent_rooms", "recent_rooms TEXT");
    addCol("subscribed", "subscribed INTEGER DEFAULT 0");

    // --- global settings (anchor week) ---
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS settings (
        key TEXT PRIMARY KEY,
        value TEXT
      );
    `);
  }

  // --- settings ---
  getSetting(key, defVal = null) {
    const row = this.db.prepare("SELECT value FROM settings WHERE key=?").get(key);
    return row?.value ?? defVal;
  }

  setSetting(key, value) {
    this.db.prepare(`
      INSERT INTO settings(key, value)
      VALUES(@key, @value)
      ON CONFLICT(key) DO UPDATE SET value=excluded.value
    `).run({ key, value: String(value) });
  }

  /**
   * Anchor week logic:
   * - anchor_monday_ymd: "YYYY-MM-DD" (понедельник)
   * - anchor_week: "numerator" | "denominator"
   */
  getWeekAnchor() {
    const anchorMondayYmd = this.getSetting("anchor_monday_ymd", null);
    const anchorWeek = this.getSetting("anchor_week", null);

    const okWeek = anchorWeek === "numerator" || anchorWeek === "denominator";
    const okDate = typeof anchorMondayYmd === "string" && /^\d{4}-\d{2}-\d{2}$/.test(anchorMondayYmd);

    if (!okWeek || !okDate) return null;
    return { anchorMondayYmd, anchorWeek };
  }

  setWeekAnchor(anchorMondayYmd, anchorWeek) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(String(anchorMondayYmd || ""))) {
      throw new Error("anchorMondayYmd должен быть YYYY-MM-DD");
    }
    if (!["numerator", "denominator"].includes(anchorWeek)) {
      throw new Error("anchorWeek должен быть numerator|denominator");
    }
    this.setSetting("anchor_monday_ymd", anchorMondayYmd);
    this.setSetting("anchor_week", anchorWeek);
  }

  // --- users ---
  getUser(chatId) {
    const row = this.db
      .prepare("SELECT group_name, week_mode, recent_teachers, recent_rooms, subscribed FROM users WHERE chat_id=?")
      .get(chatId);

    if (!row) {
      return {
        groupName: null,
        // weekMode оставляем как поле, но в боте мы его больше не используем
        weekMode: "auto",
        recentTeachers: [],
        recentRooms: [],
        subscribed: false
      };
    }

    const recentTeachers = uniqRecent(safeJsonParse(row.recent_teachers, []) || []);
    const recentRooms = uniqRecent(safeJsonParse(row.recent_rooms, []) || []);
    const subscribed = Boolean(row.subscribed);

    return {
      groupName: row.group_name || null,
      weekMode: "auto",
      recentTeachers,
      recentRooms,
      subscribed
    };
  }

  _ensureUserRow(chatId) {
    this.db.prepare(`
      INSERT INTO users(chat_id, group_name, week_mode, recent_teachers, recent_rooms, subscribed)
      VALUES(@chat_id, NULL, 'auto', '[]', '[]', 0)
      ON CONFLICT(chat_id) DO NOTHING
    `).run({ chat_id: chatId });
  }

  setUserGroup(chatId, groupName) {
    this._ensureUserRow(chatId);
    this.db.prepare(`
      UPDATE users SET group_name=@group_name WHERE chat_id=@chat_id
    `).run({ chat_id: chatId, group_name: groupName });
  }

  // week_mode оставляем на всякий, но UI для юзеров убран
  setUserWeekMode(chatId, weekMode) {
    this._ensureUserRow(chatId);
    this.db.prepare(`
      UPDATE users SET week_mode=@week_mode WHERE chat_id=@chat_id
    `).run({ chat_id: chatId, week_mode: weekMode });
  }

  pushRecentTeacher(chatId, teacherDisplay) {
    if (!teacherDisplay || !String(teacherDisplay).trim()) return;
    this._ensureUserRow(chatId);

    const row = this.db.prepare("SELECT recent_teachers FROM users WHERE chat_id=?").get(chatId);
    const arr = safeJsonParse(row?.recent_teachers, []) || [];

    const key = normText(teacherDisplay);
    const next = [String(teacherDisplay), ...arr.filter((x) => normText(x) !== key)];
    const finalArr = uniqRecent(next, 3);

    this.db.prepare("UPDATE users SET recent_teachers=? WHERE chat_id=?")
      .run(JSON.stringify(finalArr), chatId);
  }

  pushRecentRoom(chatId, roomDisplay) {
    if (!roomDisplay || !String(roomDisplay).trim()) return;
    this._ensureUserRow(chatId);

    const row = this.db.prepare("SELECT recent_rooms FROM users WHERE chat_id=?").get(chatId);
    const arr = safeJsonParse(row?.recent_rooms, []) || [];

    const key = normText(roomDisplay);
    const next = [String(roomDisplay), ...arr.filter((x) => normText(x) !== key)];
    const finalArr = uniqRecent(next, 3);

    this.db.prepare("UPDATE users SET recent_rooms=? WHERE chat_id=?")
      .run(JSON.stringify(finalArr), chatId);
  }

  toggleSubscribed(chatId) {
    this._ensureUserRow(chatId);
    const row = this.db.prepare("SELECT subscribed FROM users WHERE chat_id=?").get(chatId);
    const cur = Boolean(row?.subscribed);
    const next = cur ? 0 : 1;
    this.db.prepare("UPDATE users SET subscribed=? WHERE chat_id=?").run(next, chatId);
    return Boolean(next);
  }

  listSubscribedChatIds() {
    const rows = this.db.prepare("SELECT chat_id FROM users WHERE subscribed=1").all();
    return rows.map((r) => r.chat_id);
  }
}
