import fs from "node:fs";
import path from "node:path";
import sharp from "sharp";
import PQueue from "p-queue";
import { ensureDir, stableHash, escapeHtml } from "./util.js";

const DAY_NAMES_RU = ["Понедельник","Вторник","Среда","Четверг","Пятница","Суббота","Воскресенье"];

export class Renderer {
  constructor({ cacheDir, keepVersions = 2, concurrency = 2 }) {
    this.cacheDir = cacheDir;
    this.keepVersions = keepVersions;
    this.queue = new PQueue({ concurrency });
    this.inflight = new Map(); // key -> Promise<string>
  }

  async renderWeekPng({ snapshotVersion, cacheKey, title, weekStartDateYmd, lessonsByDay }) {
    // cacheKey — строка, уникальная для "группа/препод/аудитория"
    const key = stableHash(`${snapshotVersion}:${cacheKey}:${weekStartDateYmd}:week:v3`);
    const dir = path.join(this.cacheDir, snapshotVersion, "week");
    ensureDir(dir);

    const filePath = path.join(dir, `${key}.png`);
    if (fs.existsSync(filePath)) return filePath;

    const inflightKey = `${snapshotVersion}:${key}`;
    const existing = this.inflight.get(inflightKey);
    if (existing) return existing;

    const p = this.queue.add(async () => {
      if (fs.existsSync(filePath)) return filePath;

      const svg = buildWeekSvg({ title, weekStartDateYmd, lessonsByDay });
      const buf = await sharp(Buffer.from(svg)).png({ compressionLevel: 9 }).toBuffer();

      const tmp = `${filePath}.tmp`;
      fs.writeFileSync(tmp, buf);
      fs.renameSync(tmp, filePath);
      return filePath;
    }).finally(() => {
      this.inflight.delete(inflightKey);
    });

    this.inflight.set(inflightKey, p);
    return p;
  }
}

function buildWeekSvg({ title, weekStartDateYmd, lessonsByDay }) {
  const width = 1200;
  const headerH = 90;
  const colW = 170;
  const leftPad = 30;
  const topPad = headerH + 20;
  

  // Auto rows
  const maxLessons = Math.max(
    1,
    ...Object.values(lessonsByDay || {}).map((arr) => (Array.isArray(arr) ? arr.length : 0))
  );

  // hard cap to avoid huge PNGs
  const rows = Math.min(6, maxLessons);
  const rowH = 140;
  const height = topPad + rows * rowH + 40;

  const fontStack = "DejaVu Sans, Noto Sans, Arial, sans-serif";

  const headerTitle = `Расписание • ${title}`;
  const subtitle = `Неделя с ${weekStartDateYmd}`;

  const cols = 7;
  const tableW = colW * cols;
  const tableX = leftPad;
  const tableY = topPad;

  const cells = [];
  for (let d = 0; d < 7; d++) {
    const x = tableX + d * colW;

    cells.push(`<rect x="${x}" y="${tableY}" width="${colW}" height="${rows * rowH}" fill="white" stroke="#c8c8c8" />`);
    cells.push(`<text x="${x + 10}" y="${tableY - 10}" font-size="18" font-family="${fontStack}" fill="#111">${DAY_NAMES_RU[d]}</text>`);

    const dayLessons = Array.isArray(lessonsByDay?.[d]) ? lessonsByDay[d] : [];
    const overflow = Math.max(0, dayLessons.length - rows);

    for (let r = 0; r < rows; r++) {
      const cy = tableY + r * rowH;
      cells.push(`<rect x="${x}" y="${cy}" width="${colW}" height="${rowH}" fill="white" stroke="#e0e0e0" />`);

      let line = "";
      const l = dayLessons[r];

      if (l) {
        const subj = String(l.subject || "");
        const loc = [l.ukLabel, l.room ? `ауд. ${l.room}` : null].filter(Boolean).join(", ");
        line = loc ? `${l.time_from}-${l.time_to}  ${subj}  (${loc})` : `${l.time_from}-${l.time_to}  ${subj}`;
      } else if (r === rows - 1 && overflow > 0) {
        line = `… ещё ${overflow} пар(ы)`;
      }

      cells.push(wrapTextSvg(line, x + 10, cy + 28, colW - 20, 18, fontStack));
    }
  }

  return `<?xml version="1.0" encoding="UTF-8"?>
  <svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}">
    <rect x="0" y="0" width="${width}" height="${height}" fill="#fafafa"/>
    <text x="${leftPad}" y="44" font-size="34" font-family="${fontStack}" font-weight="700" fill="#111">${escapeXml(headerTitle)}</text>
    <text x="${leftPad}" y="74" font-size="18" font-family="${fontStack}" fill="#333">${escapeXml(subtitle)}</text>

    <rect x="${tableX}" y="${tableY}" width="${tableW}" height="${rows * rowH}" fill="white" stroke="#c8c8c8"/>
    ${cells.join("\n")}
  </svg>`;
}

function escapeXml(s) {
  return escapeHtml(String(s || ""));
}

function wrapTextSvg(text, x, y, maxWidth, lineHeight, fontStack) {
  const t = String(text || "").trim();
  if (!t) return "";

  const maxChars = Math.max(10, Math.floor(maxWidth / 9));
  const parts = [];
  let cur = "";

  for (const word of t.split(/\s+/)) {
    if ((cur + " " + word).trim().length > maxChars) {
      if (cur) parts.push(cur);
      cur = word;
    } else {
      cur = (cur ? `${cur} ${word}` : word);
    }
  }
  if (cur) parts.push(cur);

  const lines = parts.slice(0, 6);
  const tspans = lines
    .map((ln, idx) => `<tspan x="${x}" y="${y + idx * lineHeight}">${escapeXml(ln)}</tspan>`)
    .join("");
  return `<text font-size="14" font-family="${fontStack}" fill="#111">${tspans}</text>`;
}
