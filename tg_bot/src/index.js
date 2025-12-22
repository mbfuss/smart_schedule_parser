import path from "node:path";
import { mustEnv, optEnv, boolEnv, ensureDir } from "./util.js";
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

  if (boolEnv("ENABLE_INTERNAL_UPDATER", true)) {
    updater.trigger("startup").catch(() => {});
    startCron({ tz, updater, cronExpr });
  }

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

  await bot.launch();
  
  await bot.telegram.setMyCommands([
    { command: "start", description: "Запуск / главное меню" },
    { command: "menu", description: "Показать меню" },
  ]);
  process.once("SIGINT", () => bot.stop("SIGINT"));
  process.once("SIGTERM", () => bot.stop("SIGTERM"));

  console.log(`[tg-bot] started (${tz}), snapshot=${snapshotPath}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});