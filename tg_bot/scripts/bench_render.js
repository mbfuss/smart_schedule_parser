// scripts/bench_render.js
// Usage examples:
//   node scripts/bench_render.js --n 50 --types group,teacher,room --miss --hit
//   SNAPSHOT_PATH=/data/schedule.snapshot.json RENDER_CACHE_DIR=/data/render_cache node scripts/bench_render.js --n 100
//
// Flags:
//   --snapshot <path>        snapshot json path (default: process.env.SNAPSHOT_PATH or ./schedule.snapshot.json)
//   --cacheDir <path>        render cache dir (default: process.env.RENDER_CACHE_DIR or ./render_cache)
//   --n <num>                how many entities per type (default: 50)
//   --types <csv>            group,teacher,room (default: all)
//   --weekMode <auto|numerator|denominator> (default: auto)
//   --miss                   run miss benchmark (default: true)
//   --hit                    run hit benchmark (default: true)
//   --concurrency <num>      renderer queue concurrency (default: env RENDER_CONCURRENCY or 1)
//   --keepVersions <num>     renderer keep versions (default: env RENDER_KEEP_VERSIONS or 2)
//   --seed <num>             random seed (default: 42)
//
// Notes:
// - "MISS" is enforced by deleting cacheDir/<snapshotVersion>/week before the run.
// - "HIT" is immediately re-run with the same inputs.

import fs from "node:fs";
import path from "node:path";
import { performance } from "node:perf_hooks";

import { SnapshotRepo } from "../snapshot.js";
import { Renderer } from "../render.js";
import { mondayOf, ymd, ukLabelFromNum } from "../util.js";

function parseArgs(argv) {
  const out = {
    snapshot: process.env.SNAPSHOT_PATH || "./schedule.snapshot.json",
    cacheDir: process.env.RENDER_CACHE_DIR || "./render_cache",
    n: 50,
    types: ["group", "teacher", "room"],
    weekMode: "auto",
    miss: true,
    hit: true,
    concurrency: Number(process.env.RENDER_CONCURRENCY || "1"),
    keepVersions: Number(process.env.RENDER_KEEP_VERSIONS || "2"),
    seed: 42
  };

  const args = [...argv];
  while (args.length) {
    const a = args.shift();
    if (a === "--snapshot") out.snapshot = args.shift();
    else if (a === "--cacheDir") out.cacheDir = args.shift();
    else if (a === "--n") out.n = Number(args.shift());
    else if (a === "--types") out.types = String(args.shift()).split(",").map((s) => s.trim()).filter(Boolean);
    else if (a === "--weekMode") out.weekMode = String(args.shift()).trim();
    else if (a === "--miss") out.miss = true;
    else if (a === "--no-miss") out.miss = false;
    else if (a === "--hit") out.hit = true;
    else if (a === "--no-hit") out.hit = false;
    else if (a === "--concurrency") out.concurrency = Number(args.shift());
    else if (a === "--keepVersions") out.keepVersions = Number(args.shift());
    else if (a === "--seed") out.seed = Number(args.shift());
  }
  return out;
}

// deterministic RNG (mulberry32)
function rng(seed) {
  let t = seed >>> 0;
  return () => {
    t += 0x6D2B79F5;
    let x = t;
    x = Math.imul(x ^ (x >>> 15), x | 1);
    x ^= x + Math.imul(x ^ (x >>> 7), x | 61);
    return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
  };
}

function sample(arr, n, seed) {
  const r = rng(seed);
  const a = [...arr];
  // Fisher-Yates shuffle partially
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(r() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a.slice(0, Math.min(n, a.length));
}

function percentile(sorted, p) {
  if (!sorted.length) return 0;
  const idx = (sorted.length - 1) * p;
  const lo = Math.floor(idx);
  const hi = Math.ceil(idx);
  if (lo === hi) return sorted[lo];
  const w = idx - lo;
  return sorted[lo] * (1 - w) + sorted[hi] * w;
}

function summarizeMs(values) {
  const v = [...values].sort((a, b) => a - b);
  const sum = v.reduce((acc, x) => acc + x, 0);
  const avg = v.length ? sum / v.length : 0;
  return {
    count: v.length,
    avg,
    p50: percentile(v, 0.5),
    p95: percentile(v, 0.95),
    p99: percentile(v, 0.99),
    min: v[0] ?? 0,
    max: v[v.length - 1] ?? 0
  };
}

function fmtMs(x) {
  return `${Math.round(x)}ms`;
}

function printSummary(title, wallMs, cpuMs, rssMb, totalWallMs) {
  const w = summarizeMs(wallMs);
  const c = summarizeMs(cpuMs);
  const r = summarizeMs(rssMb);
  const rps = totalWallMs > 0 ? (w.count / (totalWallMs / 1000)) : 0;

  console.log(`\n=== ${title} ===`);
  console.log(`count: ${w.count}, throughput: ${rps.toFixed(2)} req/s`);
  console.log(`wall: avg ${fmtMs(w.avg)} | p50 ${fmtMs(w.p50)} | p95 ${fmtMs(w.p95)} | p99 ${fmtMs(w.p99)} | min ${fmtMs(w.min)} | max ${fmtMs(w.max)}`);
  console.log(`cpu : avg ${fmtMs(c.avg)} | p50 ${fmtMs(c.p50)} | p95 ${fmtMs(c.p95)} | p99 ${fmtMs(c.p99)} | min ${fmtMs(c.min)} | max ${fmtMs(c.max)}`);
  console.log(`rss : avg ${Math.round(r.avg)}MB | p95 ${Math.round(r.p95)}MB | max ${Math.round(r.max)}MB`);
}

function ensureCleanDir(p) {
  fs.mkdirSync(p, { recursive: true });
}

function rmIfExists(p) {
  try {
    fs.rmSync(p, { recursive: true, force: true });
  } catch {}
}

function weekActive(weekMode, semesterStartDate, dateObj) {
  if (weekMode === "numerator" || weekMode === "denominator") return weekMode;
  // auto
  const diffDays = Math.floor((dateObj.getTime() - semesterStartDate.getTime()) / (24 * 3600 * 1000));
  const weekIdx = Math.max(0, Math.floor(diffDays / 7));
  return weekIdx % 2 === 0 ? "numerator" : "denominator";
}

function lessonMatchesWeek(lessonWeek, activeWeek) {
  if (!lessonWeek) return true;
  return lessonWeek === activeWeek;
}

function buildLessonsByDayFromOccurrences(items, wa) {
  const byDay = {};
  for (let wd = 0; wd < 7; wd++) byDay[wd] = [];

  for (const it of items) {
    if (!lessonMatchesWeek(it.week, wa)) continue;
    const ukLabel = it.ukNum ? ukLabelFromNum(it.ukNum) : null;

    byDay[it.weekday].push({
      time_from: it.time_from,
      time_to: it.time_to,
      subject: it.subject,
      room: it.room || null,
      ukLabel
    });
  }

  for (let wd = 0; wd < 7; wd++) {
    byDay[wd].sort((a, b) => String(a.time_from).localeCompare(String(b.time_from)));
  }
  return byDay;
}

function buildLessonsByDayFromGroup(g, wa) {
  const lessonsByDay = {};
  for (let wd = 0; wd < 7; wd++) {
    const raw = (g.days.get(wd) || []).filter((l) => lessonMatchesWeek(l.week, wa));
    lessonsByDay[wd] = raw.map((l) => ({
      time_from: l.time_from,
      time_to: l.time_to,
      subject: l.subject,
      room: null,
      ukLabel: g.meta?.ukNum ? ukLabelFromNum(g.meta.ukNum) : null
    }));
  }
  return lessonsByDay;
}

async function runBatch({ renderer, snapshotVersion, weekStartYmd, jobs, label }) {
  const wallMs = [];
  const cpuMs = [];
  const rssMb = [];

  const t0 = performance.now();

  for (const job of jobs) {
    const cpu0 = process.cpuUsage();
    const w0 = performance.now();

    // eslint-disable-next-line no-await-in-loop
    await renderer.renderWeekPng({
      snapshotVersion,
      cacheKey: job.cacheKey,
      title: job.title,
      weekStartDateYmd: weekStartYmd,
      lessonsByDay: job.lessonsByDay
    });

    const w1 = performance.now();
    const cpu1 = process.cpuUsage(cpu0);
    const cpu = (cpu1.user + cpu1.system) / 1000; // microsec -> ms
    const rss = process.memoryUsage().rss / (1024 * 1024);

    wallMs.push(w1 - w0);
    cpuMs.push(cpu);
    rssMb.push(rss);
  }

  const t1 = performance.now();
  printSummary(label, wallMs, cpuMs, rssMb, t1 - t0);
}

async function main() {
  const cfg = parseArgs(process.argv.slice(2));

  console.log("bench config:", cfg);

  if (!fs.existsSync(cfg.snapshot)) {
    console.error(`Snapshot not found: ${cfg.snapshot}`);
    process.exit(2);
  }

  ensureCleanDir(cfg.cacheDir);

  const repo = new SnapshotRepo(cfg.snapshot);
  if (!repo.loadIfPresent()) {
    console.error("Failed to load snapshot.");
    process.exit(2);
  }

  const renderer = new Renderer({
    cacheDir: cfg.cacheDir,
    keepVersions: cfg.keepVersions,
    concurrency: cfg.concurrency
  });

  // week params
  const now = new Date();
  const weekStart = mondayOf(now);
  const weekStartYmd = ymd(weekStart);

  // IMPORTANT: semester start affects numerator/denominator in auto mode.
  // For bench we can choose a fixed semester start; adjust if you want.
  // If you want it configurable, add env/arg.
  const semesterStartDate = new Date(now.getFullYear(), 8, 1); // Sep 1 текущего года
  const wa = weekActive(cfg.weekMode, semesterStartDate, weekStart);

  const snapshotVersion = repo.snapshotVersion();
  const weekDir = path.join(cfg.cacheDir, snapshotVersion, "week");

  // Prepare jobs
  const jobs = [];

  if (cfg.types.includes("group")) {
    const groups = sample(repo.groupList, cfg.n, cfg.seed + 1);
    for (const groupName of groups) {
      const g = repo.getGroup(groupName);
      if (!g) continue;
      jobs.push({
        kind: "group",
        cacheKey: `group:${groupName}`,
        title: groupName,
        lessonsByDay: buildLessonsByDayFromGroup(g, wa)
      });
    }
  }

  if (cfg.types.includes("teacher")) {
    const teacherDisplays = Array.from(repo.teacherIndex.values()).map((v) => v.display);
    const teachers = sample(teacherDisplays, cfg.n, cfg.seed + 2);
    for (const t of teachers) {
      const items = repo.getTeacherItems(t);
      jobs.push({
        kind: "teacher",
        cacheKey: `teacher:${t}`,
        title: t,
        lessonsByDay: buildLessonsByDayFromOccurrences(items, wa)
      });
    }
  }

  if (cfg.types.includes("room")) {
    // берём "2/401" ключи, но title делаем красивый
    const roomDisplays = Array.from(repo.roomIndexByCampus.values()).map((v) => v.display); // "2/401"
    const rooms = sample(roomDisplays, cfg.n, cfg.seed + 3);
    for (const disp of rooms) {
      const [ukNum, room] = String(disp).split("/");
      const items = repo.getRoomItems(room, ukNum);
      jobs.push({
        kind: "room",
        cacheKey: `room:${ukNum}/${room}`,
        title: `${ukLabelFromNum(ukNum)}, ауд. ${room}`,
        lessonsByDay: buildLessonsByDayFromOccurrences(items, wa)
      });
    }
  }

  if (!jobs.length) {
    console.log("No jobs to run (empty selection).");
    process.exit(0);
  }

  console.log(`snapshotVersion=${snapshotVersion}, weekStart=${weekStartYmd}, weekActive=${wa}`);
  console.log(`jobs total=${jobs.length} (types=${cfg.types.join(",")}, n=${cfg.n})`);
  console.log(`cache dir: ${cfg.cacheDir}`);
  console.log(`week cache dir: ${weekDir}`);

  // MISS
  if (cfg.miss) {
    console.log("\n[miss] deleting week cache dir to force cache-miss…");
    rmIfExists(weekDir);
    await runBatch({
      renderer,
      snapshotVersion,
      weekStartYmd,
      jobs,
      label: `MISS (render) — ${cfg.types.join(",")}`
    });
  }

  // HIT
  if (cfg.hit) {
    // Ensure at least 1 render exists
    // If miss was disabled, we still need the cache populated
    if (!fs.existsSync(weekDir)) {
      console.log("\n[hit] cache empty, warming up first…");
      await runBatch({
        renderer,
        snapshotVersion,
        weekStartYmd,
        jobs: jobs.slice(0, Math.min(10, jobs.length)),
        label: `WARMUP (10)`
      });
    }

    await runBatch({
      renderer,
      snapshotVersion,
      weekStartYmd,
      jobs,
      label: `HIT (cache) — ${cfg.types.join(",")}`
    });
  }

  console.log("\nDone.");
}

main().catch((e) => {
  console.error("bench failed:", e);
  process.exit(1);
});
