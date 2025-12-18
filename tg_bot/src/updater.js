import axios from "axios";
import cron from "node-cron";
import { atomicWriteJson, boolEnv, formatLocal } from "./util.js";

export class Updater {
  constructor({
    tz,
    repo,
    snapshotPath,
    statusPath,
    parserBaseUrl,
    scheduleSourceUrl
  }) {
    this.tz = tz;
    this.repo = repo;
    this.snapshotPath = snapshotPath;
    this.statusPath = statusPath;
    this.parserBaseUrl = parserBaseUrl;
    this.scheduleSourceUrl = scheduleSourceUrl;

    this._inFlight = null;
  }

  async trigger(reason = "manual") {
    if (this._inFlight) {
      return { ok: false, error: "Обновление уже выполняется. Попробуй чуть позже." };
    }

    this._inFlight = this._runOnce(reason)
      .then(() => ({ ok: true, error: null }))
      .catch((e) => ({ ok: false, error: String(e?.message || e) }))
      .finally(() => {
        this._inFlight = null;
      });

    return this._inFlight;
  }

  async _runOnce(reason) {
    const base = this.parserBaseUrl.replace(/\/+$/, "");
    const url = `${base}/getschedule`;
    const started = Date.now();

    const status = {
      reason,
      parser_url: url,
      schedule_source_url: this.scheduleSourceUrl,
      last_attempt_at: new Date().toISOString(),
      last_success_at: null,
      last_error: null,
      duration_ms: null,
      snapshot_mtime_ms: null
    };

    try {
      const resp = await axios.get(url, {
        params: { urlSchedule: this.scheduleSourceUrl },
        timeout: 180_000,
        validateStatus: () => true
      });

      if (resp.status !== 200) {
        const body = typeof resp.data === "string" ? resp.data : JSON.stringify(resp.data);
        throw new Error(`Parser HTTP ${resp.status}: ${body.slice(0, 500)}`);
      }

      atomicWriteJson(this.snapshotPath, resp.data);

      this.repo.loadIfPresent();

      status.last_success_at = new Date().toISOString();
      status.duration_ms = Date.now() - started;
      status.snapshot_mtime_ms = this.repo.mtimeMs;

      atomicWriteJson(this.statusPath, status);

      console.log(`[updater] ok (${reason}) at ${formatLocal(Date.now(), this.tz)}`);
    } catch (e) {
      status.last_error = String(e?.message || e);
      status.duration_ms = Date.now() - started;
      atomicWriteJson(this.statusPath, status);
      console.error(`[updater] fail (${reason}):`, status.last_error);
      throw e;
    }
  }
}

export function startCron({ tz, updater, cronExpr }) {
  if (!boolEnv("ENABLE_INTERNAL_UPDATER", true)) return;
  cron.schedule(cronExpr, () => updater.trigger("cron").catch(() => {}), { timezone: tz });
  console.log(`[updater] scheduled: "${cronExpr}" (${tz})`);
}