import { Telegraf, Markup } from "telegraf";
import { LRUCache } from "lru-cache";
import fs from "node:fs";
import {
  escapeHtml,
  formatLocal,
  mondayOf,
  ymd,
  addDays,
  parseUkRoom,
  ukLabelFromNum
} from "./util.js";

const DAY_NAMES_RU = ["Понедельник","Вторник","Среда","Четверг","Пятница","Суббота","Воскресенье"];

function kbMain() {
  return Markup.inlineKeyboard([
    [Markup.button.callback("Сегодня", "day:today"), Markup.button.callback("Завтра", "day:tomorrow")],
    [Markup.button.callback("Неделя", "week:text"), Markup.button.callback("🖼 Картинка недели", "week:pic")],
    [Markup.button.callback("Преподаватель", "search:teacher"), Markup.button.callback("Аудитория", "search:room")],
    [Markup.button.callback("Сменить группу", "group:change")]
  ]);
}

function kbList(prefix, values) {
  return Markup.inlineKeyboard(values.slice(0, 10).map((v) => [Markup.button.callback(v, `${prefix}:${v}`)]));
}

function readJsonIfExists(p) {
  try {
    if (!fs.existsSync(p)) return null;
    return JSON.parse(fs.readFileSync(p, "utf8"));
  } catch {
    return null;
  }
}

function parseRoomFromSubject(subject) {
  const s = String(subject || "");
  const m = s.match(/(?:^|[,\s;])(?:ауд\.?\s*)?(\d{1,4}[а-яa-z]?)(?:$|[,\s;])/iu);
  return m ? String(m[1]).toUpperCase() : null;
}

function parseUkNumFromSubject(subject) {
  const s = String(subject || "");
  const m = s.match(/УК\s*№?\s*(\d+)/iu);
  return m ? String(m[1]) : null;
}

export function createBot({
  token,
  tz,
  repo,
  db,
  renderer,
  semesterStartDate,
  snapshotPath,
  statusPath,
  updater,
  updatePassword
}) {
  const bot = new Telegraf(token);
  const textCache = new LRUCache({ max: 4096, ttl: 30_000 });

  // chat FSM-lite
  const state = new Map(); // chatId -> { mode, ukNum?, tries?, expiresAt?, purpose? }
  const lockState = new Map(); // chatId -> lockUntilMs

  const AUTH_TTL_MS = 2 * 60 * 1000;
  const MAX_TRIES = 5;
  const LOCK_MS = 15 * 60 * 1000;

  const updatedAtText = () => {
    const st = fs.existsSync(snapshotPath) ? fs.statSync(snapshotPath) : null;
    const ms = st ? st.mtimeMs : Date.now();
    return formatLocal(ms, tz);
  };

  const weekActive = (weekMode, dateObj) => {
    if (weekMode === "numerator" || weekMode === "denominator") return weekMode;
    const diffDays = Math.floor((dateObj.getTime() - semesterStartDate.getTime()) / (24 * 3600 * 1000));
    const weekIdx = Math.max(0, Math.floor(diffDays / 7));
    return weekIdx % 2 === 0 ? "numerator" : "denominator";
  };

  function lessonMatchesWeek(lessonWeek, activeWeek) {
    if (!lessonWeek) return true;
    return lessonWeek === activeWeek;
  }

  async function ensureSnapshotLoaded(ctx) {
    if (repo.groups.size > 0) return true;
    const ok = repo.loadIfPresent();
    if (ok) return true;
    await ctx.reply("Пока нет данных snapshot. Подожди обновление или попроси админа /update.");
    return false;
  }

  function isLocked(chatId) {
    const until = lockState.get(chatId) || 0;
    return until > Date.now();
  }

  function lockLeftSeconds(chatId) {
    const until = lockState.get(chatId) || 0;
    return Math.max(0, Math.ceil((until - Date.now()) / 1000));
  }

  function formatGroupDayText(groupName, dateObj, dayShift, weekMode) {
    const g = repo.getGroup(groupName);
    if (!g) return `Группа не найдена: ${escapeHtml(groupName)}`;

    const d = addDays(dateObj, dayShift);
    const weekday = (d.getDay() + 6) % 7;
    const dateText = ymd(d);
    const wa = weekActive(weekMode, d);

    const lessons = g.days.get(weekday) || [];
    const filtered = lessons.filter((l) => lessonMatchesWeek(l.week, wa));

    const ukLabel = g.meta?.ukNum ? ukLabelFromNum(g.meta.ukNum) : (g.meta?.campusName || "—");

    let out =
      `📚 <b>${escapeHtml(groupName)}</b>  🏢 <b>${escapeHtml(ukLabel)}</b>\n` +
      `📅 ${DAY_NAMES_RU[weekday]} • ${escapeHtml(dateText)}\n` +
      `🔁 Неделя: <b>${wa === "numerator" ? "числитель" : "знаменатель"}</b>\n` +
      `🕒 Обновлено: ${escapeHtml(updatedAtText())}\n\n`;

    if (filtered.length === 0) return out + "✅ Пар нет (по выбранной неделе).";

    for (const l of filtered) {
      out += `• <b>${escapeHtml(l.time_from)}-${escapeHtml(l.time_to)}</b> — ${escapeHtml(l.subject)}\n`;
    }
    return out;
  }

  function formatOccurrencesWeek(items, weekMode, dateObj) {
    const weekStart = mondayOf(dateObj);
    const wa = weekActive(weekMode, weekStart);

    const byDay = new Map();
    for (let wd = 0; wd < 7; wd++) byDay.set(wd, []);

    for (const it of items) {
      if (!lessonMatchesWeek(it.week, wa)) continue;
      byDay.get(it.weekday)?.push(it);
    }

    let out =
      `📆 Неделя с <b>${escapeHtml(ymd(weekStart))}</b>\n` +
      `🔁 Неделя: <b>${wa === "numerator" ? "числитель" : "знаменатель"}</b>\n` +
      `🕒 Обновлено: ${escapeHtml(updatedAtText())}\n\n`;

    for (let wd = 0; wd < 7; wd++) {
      const list = byDay.get(wd) || [];
      if (list.length === 0) continue;

      list.sort((a, b) => String(a.time_from).localeCompare(String(b.time_from)));
      out += `— <b>${DAY_NAMES_RU[wd]}</b>\n`;
      for (const x of list) {
        const uk = x.ukNum ? ukLabelFromNum(x.ukNum) : null;
        const room = x.room ? `ауд. ${x.room}` : null;
        const loc = [uk, room].filter(Boolean).join(", ");
        out += `  • <b>${escapeHtml(x.time_from)}-${escapeHtml(x.time_to)}</b> — ${escapeHtml(x.subject)}\n`;
        out += `    <i>${escapeHtml(x.group)}${loc ? ` • ${escapeHtml(loc)}` : ""}</i>\n`;
      }
      out += "\n";
    }

    return out.trim() || "Пусто";
  }

  function formatStatus() {
    const st = readJsonIfExists(statusPath);
    if (!st) return "Статус ещё не сформирован (status.json отсутствует).";

    const lastAttempt = st.last_attempt_at ? new Date(st.last_attempt_at).getTime() : null;
    const lastSuccess = st.last_success_at ? new Date(st.last_success_at).getTime() : null;

    const lines = [];
    lines.push(`📦 Snapshot: ${escapeHtml(updatedAtText())}`);
    if (st.reason) lines.push(`🧩 Причина: ${escapeHtml(st.reason)}`);
    if (lastAttempt) lines.push(`🕵️ Последняя попытка: ${escapeHtml(formatLocal(lastAttempt, tz))}`);
    if (lastSuccess) lines.push(`✅ Успешно: ${escapeHtml(formatLocal(lastSuccess, tz))}`);
    if (st.duration_ms != null) lines.push(`⏱️ Длительность: ${escapeHtml(String(st.duration_ms))} ms`);
    if (st.last_error) lines.push(`❌ Ошибка: ${escapeHtml(String(st.last_error)).slice(0, 900)}`);
    return lines.join("\n");
  }

  // ---- /update & /status with same password ----
  function beginPasswordFlow(ctx, purpose) {
    if (!updatePassword) {
      ctx.reply("Доступ запрещён (UPDATE_PASSWORD не задан).");
      return;
    }
    const chatId = ctx.chat.id;
    if (isLocked(chatId)) {
      ctx.reply(`Слишком много попыток. Подожди ${lockLeftSeconds(chatId)} сек.`);
      return;
    }
    state.set(chatId, { mode: "await_password", purpose, tries: 0, expiresAt: Date.now() + AUTH_TTL_MS });
    ctx.reply("Введите пароль одним сообщением. /cancel для отмены");
  }

  bot.command("update", (ctx) => beginPasswordFlow(ctx, "update"));
  bot.command("status", (ctx) => beginPasswordFlow(ctx, "status"));

  bot.command("cancel", async (ctx) => {
    state.delete(ctx.chat.id);
    await ctx.reply("Ок, отменил.");
  });

  async function handlePasswordIfNeeded(ctx) {
    const chatId = ctx.chat.id;
    const st = state.get(chatId);
    if (!st || st.mode !== "await_password") return false;

    if (st.expiresAt < Date.now()) {
      state.delete(chatId);
      await ctx.reply("Время ожидания пароля истекло. Повтори /update или /status");
      return true;
    }

    const text = String(ctx.message?.text || "").trim();
    if (!text) return true;

    if (isLocked(chatId)) {
      state.delete(chatId);
      await ctx.reply(`Слишком много попыток. Подожди ${lockLeftSeconds(chatId)} сек.`);
      return true;
    }

    if (text !== updatePassword) {
      st.tries += 1;
      if (st.tries >= MAX_TRIES) {
        state.delete(chatId);
        lockState.set(chatId, Date.now() + LOCK_MS);
        await ctx.reply("❌ Неверный пароль. Блокировка на 15 минут.");
        return true;
      }
      await ctx.reply(`❌ Неверный пароль. Осталось попыток: ${MAX_TRIES - st.tries}. /cancel`);
      return true;
    }

    // OK password
    const purpose = st.purpose;
    state.delete(chatId);

    if (purpose === "status") {
      await ctx.reply(formatStatus(), { parse_mode: "HTML", ...kbMain() });
      return true;
    }

    // purpose === "update"
    await ctx.reply("Пароль принят. Запускаю обновление…");
    const res = await updater.trigger("manual");
    if (res.ok) {
      renderer.cleanupOldVersions();
      await ctx.reply(`✅ Обновление выполнено.\n🕒 ${escapeHtml(updatedAtText())}`, { parse_mode: "HTML", ...kbMain() });
    } else {
      await ctx.reply(`❌ Не удалось обновить: ${escapeHtml(res.error)}`, { parse_mode: "HTML", ...kbMain() });
    }
    return true;
  }

  // ---- UI actions ----
  bot.start(async (ctx) => {
    await ensureSnapshotLoaded(ctx);
    const u = db.getUser(ctx.chat.id);
    if (!u.groupName) {
      state.set(ctx.chat.id, { mode: "group" });
      await ctx.reply("Напиши группу (например: Б-Э-301).");
      return;
    }
    await ctx.reply(
      `Текущая группа: <b>${escapeHtml(u.groupName)}</b>\nОбновлено: ${escapeHtml(updatedAtText())}`,
      { parse_mode: "HTML", ...kbMain() }
    );
  });

  bot.action("group:change", async (ctx) => {
    await ctx.answerCbQuery();
    state.set(ctx.chat.id, { mode: "group" });
    await ctx.reply("Напиши группу текстом (например: Б-Э-301).");
  });

  bot.action("search:teacher", async (ctx) => {
    await ctx.answerCbQuery();
    state.set(ctx.chat.id, { mode: "teacher" });
    await ctx.reply("Введи фамилию преподавателя (можно с ошибкой), например: Иванов");
  });

  bot.action("search:room", async (ctx) => {
    await ctx.answerCbQuery();
    await ensureSnapshotLoaded(ctx);

    if (!repo.ukNums.length) {
      state.set(ctx.chat.id, { mode: "room_any" });
      await ctx.reply("Введи аудиторию (например: 401) или УК/ауд (например: 2/401).");
      return;
    }

    const ukButtons = repo.ukNums.map((n) => Markup.button.callback(ukLabelFromNum(n), `room:uk:${n}`));
    const rows = [];
    for (let i = 0; i < ukButtons.length; i += 2) rows.push(ukButtons.slice(i, i + 2));
    rows.push([Markup.button.callback("Любой УК", "room:any")]);

    await ctx.reply("Выбери учебный корпус (УК) или нажми «Любой УК».", Markup.inlineKeyboard(rows));
  });

  bot.action(/^room:uk:(\d)$/i, async (ctx) => {
    await ctx.answerCbQuery();
    const ukNum = ctx.match[1];
    state.set(ctx.chat.id, { mode: "room_in_uk", ukNum });
    await ctx.reply(`Ок. Введи номер аудитории для ${ukLabelFromNum(ukNum)} (например: 401)\nМожно сразу: ${ukNum}/401`);
  });

  bot.action("room:any", async (ctx) => {
    await ctx.answerCbQuery();
    state.set(ctx.chat.id, { mode: "room_any" });
    await ctx.reply("Введи аудиторию (например: 401) или УК/ауд (например: 2/401).");
  });

  bot.action(/^group:set:(.+)$/i, async (ctx) => {
    const groupName = ctx.match[1];
    db.setUserGroup(ctx.chat.id, groupName);
    await ctx.answerCbQuery("Ок");
    state.delete(ctx.chat.id);
    await ctx.reply(`Сохранил группу: <b>${escapeHtml(groupName)}</b>`, { parse_mode: "HTML", ...kbMain() });
  });

  bot.action(/^teacher:pick:(.+)$/i, async (ctx) => {
    await ctx.answerCbQuery();
    const teacherDisplay = ctx.match[1];
    const u = db.getUser(ctx.chat.id);
    const items = repo.getTeacherItems(teacherDisplay);
    const msg = `<b>${escapeHtml(teacherDisplay)}</b>\n\n` + formatOccurrencesWeek(items, u.weekMode, new Date());
    await ctx.reply(msg, { parse_mode: "HTML", ...kbMain() });
  });

  bot.action(/^room:pick:(.+)$/i, async (ctx) => {
    await ctx.answerCbQuery();
    const payload = ctx.match[1]; // "2/401" or "401"
    const u = db.getUser(ctx.chat.id);

    let ukNum = null;
    let room = payload;

    const parsed = parseUkRoom(payload);
    if (parsed) {
      ukNum = parsed.ukNum;
      room = parsed.room;
    }

    const items = repo.getRoomItems(room, ukNum);
    const title = ukNum ? `${ukLabelFromNum(ukNum)}, ауд. ${room}` : `Аудитория ${room}`;
    const msg = `<b>${escapeHtml(title)}</b>\n\n` + formatOccurrencesWeek(items, u.weekMode, new Date());
    await ctx.reply(msg, { parse_mode: "HTML", ...kbMain() });
  });

  bot.action(/^day:(today|tomorrow)$/i, async (ctx) => {
    await ctx.answerCbQuery();
    await ensureSnapshotLoaded(ctx);

    const u = db.getUser(ctx.chat.id);
    if (!u.groupName) return ctx.reply("Сначала выбери группу: /start");

    const shift = ctx.match[1] === "tomorrow" ? 1 : 0;
    const ver = repo.snapshotVersion();
    const key = `day:${u.groupName}:${shift}:${u.weekMode}:${ver}`;

    const cached = textCache.get(key);
    if (cached) return ctx.reply(cached, { parse_mode: "HTML", ...kbMain() });

    const msg = formatGroupDayText(u.groupName, new Date(), shift, u.weekMode);
    textCache.set(key, msg);
    return ctx.reply(msg, { parse_mode: "HTML", ...kbMain() });
  });

  bot.action("week:text", async (ctx) => {
    await ctx.answerCbQuery();
    await ensureSnapshotLoaded(ctx);

    const u = db.getUser(ctx.chat.id);
    if (!u.groupName) return ctx.reply("Сначала выбери группу: /start");

    const ver = repo.snapshotVersion();
    const key = `week:text:${u.groupName}:${u.weekMode}:${ver}`;
    const cached = textCache.get(key);
    if (cached) return ctx.reply(cached, { parse_mode: "HTML", ...kbMain() });

    const g = repo.getGroup(u.groupName);
    if (!g) return ctx.reply("Группа не найдена в snapshot.");

    const weekStart = mondayOf(new Date());
    const wa = weekActive(u.weekMode, weekStart);
    const ukLabel = g.meta?.ukNum ? ukLabelFromNum(g.meta.ukNum) : (g.meta?.campusName || "—");

    let out =
      `📚 <b>${escapeHtml(u.groupName)}</b>  🏢 <b>${escapeHtml(ukLabel)}</b>\n` +
      `📆 Неделя с <b>${escapeHtml(ymd(weekStart))}</b>\n` +
      `🔁 Неделя: <b>${wa === "numerator" ? "числитель" : "знаменатель"}</b>\n` +
      `🕒 Обновлено: ${escapeHtml(updatedAtText())}\n\n`;

    for (let wd = 0; wd < 7; wd++) {
      const lessons = (g.days.get(wd) || []).filter((l) => lessonMatchesWeek(l.week, wa));
      if (!lessons.length) continue;
      out += `— <b>${DAY_NAMES_RU[wd]}</b>\n`;
      for (const l of lessons) out += `  • <b>${escapeHtml(l.time_from)}-${escapeHtml(l.time_to)}</b> — ${escapeHtml(l.subject)}\n`;
      out += "\n";
    }

    out = out.trim() || "Пусто";
    textCache.set(key, out);
    return ctx.reply(out, { parse_mode: "HTML", ...kbMain() });
  });

  bot.action("week:pic", async (ctx) => {
    await ctx.answerCbQuery();
    await ensureSnapshotLoaded(ctx);

    const u = db.getUser(ctx.chat.id);
    if (!u.groupName) return ctx.reply("Сначала выбери группу: /start");

    const g = repo.getGroup(u.groupName);
    if (!g) return ctx.reply("Группа не найдена в snapshot.");

    const now = new Date();
    const weekStart = mondayOf(now);
    const weekStartYmd = ymd(weekStart);
    const wa = weekActive(u.weekMode, weekStart);
    const ver = repo.snapshotVersion();

    const lessonsByDay = {};
    for (let wd = 0; wd < 7; wd++) {
      const raw = (g.days.get(wd) || []).filter((l) => lessonMatchesWeek(l.week, wa));
      lessonsByDay[wd] = raw.map((l) => {
        const room = parseRoomFromSubject(l.subject);
        const ukFromSubj = parseUkNumFromSubject(l.subject);
        const ukNum = ukFromSubj || g.meta?.ukNum || null;
        return {
          time_from: l.time_from,
          time_to: l.time_to,
          subject: l.subject,
          room,
          ukLabel: ukNum ? ukLabelFromNum(ukNum) : null
        };
      });
    }

    await ctx.reply("Генерирую картинку…");
    try {
      const filePath = await renderer.renderWeekPng({
        snapshotVersion: ver,
        groupName: u.groupName,
        weekStartDateYmd: weekStartYmd,
        lessonsByDay
      });

      return ctx.replyWithPhoto(
        { source: filePath },
        { caption: `🖼 ${u.groupName} • неделя с ${weekStartYmd}\nОбновлено: ${updatedAtText()}`, ...kbMain() }
      );
    } catch (e) {
      return ctx.reply(`Не удалось сгенерировать картинку: ${String(e?.message || e)}`, kbMain());
    }
  });

  // ---- text input router ----
  bot.on("text", async (ctx) => {
    if (await handlePasswordIfNeeded(ctx)) return;

    await ensureSnapshotLoaded(ctx);

    const chatId = ctx.chat.id;
    const st = state.get(chatId);
    const text = String(ctx.message.text || "").trim();
    if (!text) return;

    // direct "2/401"
    const direct = parseUkRoom(text);
    if (direct) {
      const u = db.getUser(chatId);
      const items = repo.getRoomItems(direct.room, direct.ukNum);
      if (!items.length) {
        const sugg = repo.suggestRooms(direct.room, direct.ukNum, 10);
        return ctx.reply(
          `Не нашёл ${ukLabelFromNum(direct.ukNum)}, ауд. ${escapeHtml(direct.room)}.\nПохожие:`,
          kbList("room:pick", sugg)
        );
      }
      const title = `${ukLabelFromNum(direct.ukNum)}, ауд. ${direct.room}`;
      const msg = `<b>${escapeHtml(title)}</b>\n\n` + formatOccurrencesWeek(items, u.weekMode, new Date());
      return ctx.reply(msg, { parse_mode: "HTML", ...kbMain() });
    }

    if (st?.mode === "teacher") {
      const teachers = repo.suggestTeachers(text, 10);
      if (!teachers.length) return ctx.reply("Не нашёл преподавателя. Попробуй иначе.");
      return ctx.reply("Выбери преподавателя:", kbList("teacher:pick", teachers));
    }

    if (st?.mode === "room_in_uk") {
      const ukNum = st.ukNum;
      const room = String(text).toUpperCase();
      const items = repo.getRoomItems(room, ukNum);
      if (!items.length) {
        const sugg = repo.suggestRooms(room, ukNum, 10);
        return ctx.reply(
          `Не нашёл ${ukLabelFromNum(ukNum)}, ауд. ${escapeHtml(room)}.\nПохожие:`,
          kbList("room:pick", sugg)
        );
      }
      const u = db.getUser(chatId);
      const title = `${ukLabelFromNum(ukNum)}, ауд. ${room}`;
      const msg = `<b>${escapeHtml(title)}</b>\n\n` + formatOccurrencesWeek(items, u.weekMode, new Date());
      return ctx.reply(msg, { parse_mode: "HTML", ...kbMain() });
    }

    if (st?.mode === "room_any") {
      const room = String(text).toUpperCase();
      const items = repo.getRoomItems(room, null);
      if (!items.length) {
        const sugg = repo.suggestRooms(room, null, 10);
        return ctx.reply(`Не нашёл аудиторию ${escapeHtml(room)}.\nПохожие:`, kbList("room:pick", sugg));
      }
      const u = db.getUser(chatId);
      const msg = `<b>Аудитория ${escapeHtml(room)}</b>\n\n` + formatOccurrencesWeek(items, u.weekMode, new Date());
      return ctx.reply(msg, { parse_mode: "HTML", ...kbMain() });
    }

    // default: group search
    const candidates = repo.findGroups(text, 10);
    if (!candidates.length) return ctx.reply("Не нашёл группу. Попробуй иначе.");
    return ctx.reply("Выбери группу:", kbList("group:set", candidates));
  });

  // hot reload
  setInterval(() => {
    try {
      if (repo.maybeReload()) {
        textCache.clear();
        renderer.cleanupOldVersions();
      }
    } catch {}
  }, 10_000);

  return bot;
}
