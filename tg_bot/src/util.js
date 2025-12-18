import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";

export function mustEnv(name) {
  const v = process.env[name];
  if (!v || !v.trim()) throw new Error(`Missing env: ${name}`);
  return v.trim();
}

export function optEnv(name, defVal) {
  const v = process.env[name];
  return v && v.trim() ? v.trim() : defVal;
}

export function boolEnv(name, defVal = false) {
  const v = process.env[name];
  if (!v || !v.trim()) return defVal;
  return ["1", "true", "yes", "y", "on"].includes(v.trim().toLowerCase());
}

export function ensureDir(dirPath) {
  fs.mkdirSync(dirPath, { recursive: true });
}

export function atomicWriteText(filePath, textUtf8) {
  ensureDir(path.dirname(filePath));
  const tmp = `${filePath}.tmp`;
  fs.writeFileSync(tmp, textUtf8, "utf8");
  fs.renameSync(tmp, filePath);
}

export function atomicWriteJson(filePath, obj) {
  atomicWriteText(filePath, JSON.stringify(obj, null, 2));
}

export function normText(s) {
  return String(s || "")
    .toLowerCase()
    .trim()
    .replace(/\s+/g, " ")
    .replace(/ё/g, "е");
}

export function escapeHtml(s) {
  return String(s || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

export function stableHash(input) {
  return crypto.createHash("sha1").update(String(input)).digest("hex").slice(0, 12);
}

export function fileExists(p) {
  try {
    fs.accessSync(p, fs.constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

export function safeStat(p) {
  try {
    return fs.statSync(p);
  } catch {
    return null;
  }
}

export function formatLocal(tsMs, tz) {
  return new Date(tsMs).toLocaleString("ru-RU", { timeZone: tz });
}

export function ymd(dateObj) {
  const y = dateObj.getFullYear();
  const m = String(dateObj.getMonth() + 1).padStart(2, "0");
  const d = String(dateObj.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export function addDays(dateObj, days) {
  return new Date(dateObj.getTime() + days * 24 * 3600 * 1000);
}

export function mondayOf(dateObj) {
  const jsDay = dateObj.getDay();
  const monBased = (jsDay + 6) % 7;
  return addDays(dateObj, -monBased);
}

export function levenshtein(a, b) {
  const s = normText(a);
  const t = normText(b);
  if (s === t) return 0;
  if (!s) return t.length;
  if (!t) return s.length;

  const n = s.length;
  const m = t.length;
  const dp = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
  for (let i = 0; i <= n; i++) dp[i][0] = i;
  for (let j = 0; j <= m; j++) dp[0][j] = j;

  for (let i = 1; i <= n; i++) {
    const si = s.charCodeAt(i - 1);
    for (let j = 1; j <= m; j++) {
      const cost = si === t.charCodeAt(j - 1) ? 0 : 1;
      dp[i][j] = Math.min(
        dp[i - 1][j] + 1,
        dp[i][j - 1] + 1,
        dp[i - 1][j - 1] + cost
      );
    }
  }
  return dp[n][m];
}


export function suggestClosest(query, candidates, limit = 10) {
  const q = normText(query);
  if (!q) return candidates.slice(0, limit);

  const scored = candidates.map((c) => {
    const cn = normText(c);
    if (cn.includes(q)) return { c, score: 0 };
    const dist = levenshtein(q, cn);
    const lenPenalty = Math.abs(cn.length - q.length);
    return { c, score: dist * 10 + lenPenalty };
  });

  scored.sort((x, y) => x.score - y.score);
  return scored.slice(0, limit).map((x) => x.c);
}

/**
 * Parse "2/401" or "2-401" or "2 401".
 */
export function parseUkRoom(input) {
  const s = String(input || "").trim();
  const m = s.match(/^\s*(\d)\s*[/\- ]\s*([0-9a-zа-я]+)\s*$/iu);
  if (!m) return null;
  return { ukNum: m[1], room: String(m[2]).toUpperCase() };
}

export function ukNumFromCampusName(campusName) {
  const m = String(campusName || "").match(/(\d+)/);
  return m ? m[1] : null;
}

export function ukLabelFromNum(ukNum) {
  return ukNum ? `УК ${ukNum}` : "УК ?";
}
