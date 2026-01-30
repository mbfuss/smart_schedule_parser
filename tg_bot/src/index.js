import path from "node:path";
import { mustEnv, optEnv, boolEnv, ensureDir, escapeHtml, formatLocal } from "./util.js";
import { DB } from "./db.js";
import { SnapshotRepo } from "./snapshot.js";
import { Renderer } from "./render.js";
import { createBot } from "./bot.js";
import { Updater, startCron } from "./updater.js";

function parseDateYmd(s) {
  const [y, m, d] = s.split("-").map(Number);
  return new Date(y, m - 1, d);
}

async function main() {
  const token = mustEnv("BOT_TOKEN");
  const tz = optEnv("TZ", "Europe/Saratov");

  const snapshotPath = optEnv("SNAPSHOT_PATH", "/data/schedule.snapshot.json");
  const statusPath = optEnv("STATUS_PATH", "/data/status.json");
  const dbPath = optEnv("DB_PATH", "/data/bot.db");

  const cacheDir = optEnv("RENDER_CACHE_DIR", "/data/render_cache");
  ensureDir(path.dirname(snapshotPath));
  ensureDir(cacheDir);

  const repo = new SnapshotRepo(snapshotPath);
  repo.loadIfPresent();

  const db = new DB(dbPath);
  db.init();

  const renderer = new Renderer({
    cacheDir,
    keepVersions: Number(optEnv("RENDER_KEEP_VERSIONS", "2")),
    concurrency: Number(optEnv("RENDER_CONCURRENCY", "2"))
  });

  const semesterStart = parseDateYmd(optEnv("SEMESTER_START_DATE", "2025-09-01"));

  const parserBaseUrl = optEnv("PARSER_BASE_URL", "http://pdf-parser:8080");
  const scheduleSourceUrl = optEnv("SCHEDULE_SOURCE_URL", "www.vavilovsar.ru/ucheba/raspisanie-zanyatii");
  const cronExpr = optEnv("UPDATE_CRON", "0 3 * * *");

  const updater = new Updater({
    tz,
    repo,
    snapshotPath,
    statusPath,
    parserBaseUrl,
    scheduleSourceUrl
  });

  const updatePassword = optEnv("UPDATE_PASSWORD", "").trim() || null;

  const bot = createBot({
    token,
    tz,
    repo,
    db,
    renderer,
    semesterStartDate: semesterStart,
    snapshotPath,
    statusPath,
    updater,
    updatePassword
  });

  // --- notify subscribers on every finished update ---
  updater.onFinish(async ({ ok, status }) => {
    const subs = db.listSubscribedChatIds();
    if (!subs.length) return;

    const whenAttempt = status?.last_attempt_at ? new Date(status.last_attempt_at).getTime() : Date.now();
    const dur = status?.duration_ms != null ? `${status.duration_ms} ms` : "—";
    const reason = status?.reason || "—";

    let msg = "";
    if (ok) {
      const whenOk = status?.last_success_at ? new Date(status.last_success_at).getTime() : whenAttempt;
      msg =
        `✅ <b>Обновление выполнено</b>\n` +
        `🧩 Причина: <b>${escapeHtml(reason)}</b>\n` +
        `🕒 Время: ${escapeHtml(formatLocal(whenOk, tz))}\n` +
        `⏱️ Длительность: ${escapeHtml(dur)}\n` +
        `Команда: /status`;
    } else {
      const err = String(status?.last_error || "unknown").slice(0, 900);
      msg =
        `❌ <b>Обновление не удалось</b>\n` +
        `🧩 Причина: <b>${escapeHtml(reason)}</b>\n` +
        `🕒 Время: ${escapeHtml(formatLocal(whenAttempt, tz))}\n` +
        `⏱️ Длительность: ${escapeHtml(dur)}\n` +
        `❌ Ошибка: <code>${escapeHtml(err)}</code>\n` +
        `Команда: /status`;
    }

    for (const chatId of subs) {
      try {
        await bot.telegram.sendMessage(chatId, msg, { parse_mode: "HTML" });
      } catch {}
    }
  });

  if (boolEnv("ENABLE_INTERNAL_UPDATER", true)) {
    updater.trigger("startup").catch(() => {});
    startCron({ tz, updater, cronExpr });
  }

  await bot.launch();

  await bot.telegram.setMyCommands([
    { command: "start", description: "Запуск / главное меню" },
    { command: "menu", description: "Показать меню" },
    { command: "status", description: "Статус обновления (пароль)" },
    { command: "update", description: "Запустить обновление (пароль)" },
    { command: "subscribe", description: "Подписка на результат обновлений (пароль)" },
    { command: "setweek", description: "Админ: задать текущую неделю (пароль)" }
  ]);

  process.once("SIGINT", () => bot.stop("SIGINT"));
  process.once("SIGTERM", () => bot.stop("SIGTERM"));

  console.log(`[tg-bot] started (${tz}), snapshot=${snapshotPath}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
